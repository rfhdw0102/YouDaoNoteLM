import client, { doRefreshToken } from './client';
import type { ApiResponse } from './auth';
import type { SearchResultItem } from './search';
import { getChatErrorMessage } from '../utils/error';

// ============ Request/Response Types ============

export interface ConversationData {
  id: number;
  title: string;
  notebook_id: number;
  created_at: string;
  updated_at: string;
}

export interface MessageData {
  id: number;
  role: 'user' | 'assistant' | 'system';
  content: string;
  metadata: {
    references?: ReferenceData[];
  } | null;
  created_at: string;
}

export interface ReferenceData {
  source_id: number;
  source_name: string;
  parent_block_id: number;
  chunk_content: string;
  score: number;
}

export interface StreamEvent {
  type:
    | 'token' | 'reference' | 'done' | 'error' | 'message' | 'title' | 'tool_call' | 'tool_result'
    | 'search_started' | 'search_results' | 'search_busy'
    | 'generation_started' | 'generation_result';
  content: string;
  data: ReferenceData[] | number | string | null;
}

// ============ API Functions ============

// 1. Create conversation
export async function createConversation(
  notebookId: number,
  title?: string
): Promise<ApiResponse<{ id: number }>> {
  const res = await client.post<ApiResponse<{ id: number }>>(`/chat/conversations`, {
    notebook_id: notebookId,
    title: title || '新对话',
  });
  return res.data;
}

// 2. List conversations for a notebook
export async function listConversations(
  notebookId: number
): Promise<ApiResponse<ConversationData[]>> {
  const res = await client.get<ApiResponse<ConversationData[]>>(
    `/chat/notebooks/${notebookId}/conversations`
  );
  return res.data;
}

// 3. Get conversation detail
export async function getConversation(
  conversationId: number
): Promise<ApiResponse<ConversationData>> {
  const res = await client.get<ApiResponse<ConversationData>>(
    `/chat/conversations/${conversationId}`
  );
  return res.data;
}

// 4. Update conversation title
export async function updateConversation(
  conversationId: number,
  title: string
): Promise<ApiResponse> {
  const res = await client.put<ApiResponse>(`/chat/conversations/${conversationId}`, {
    title,
  });
  return res.data;
}

// 5. Delete conversation
export async function deleteConversation(
  conversationId: number
): Promise<ApiResponse> {
  const res = await client.delete<ApiResponse>(`/chat/conversations/${conversationId}`);
  return res.data;
}

// 6. Get message history
export async function getMessages(
  conversationId: number
): Promise<ApiResponse<MessageData[]>> {
  const res = await client.get<ApiResponse<MessageData[]>>(
    `/chat/conversations/${conversationId}/messages`
  );
  return res.data;
}

// Helper: check if response indicates token error
async function isTokenErrorResponse(response: Response): Promise<boolean> {
  // HTTP 401/403 typically indicates auth issues
  if (response.status === 401 || response.status === 403) {
    return true;
  }
  // Check response body for token error codes (1005/1006)
  if (response.headers.get('content-type')?.includes('application/json')) {
    try {
      const data = await response.clone().json();
      return data && (data.code === 1005 || data.code === 1006);
    } catch {
      return false;
    }
  }
  return false;
}

// 7. Send message (streaming) - returns Response and AbortController
export async function sendMessage(
  conversationId: number,
  content: string,
  sourceIds?: number[],
  llmConfigId?: number
): Promise<{ response: Response; abortController: AbortController }> {
  const abortController = new AbortController();

  const makeRequest = (token: string) =>
    fetch(`/api/v1/chat/conversations/${conversationId}/messages`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        content,
        source_ids: sourceIds || [],
        llm_config_id: llmConfigId || 0,
      }),
      signal: abortController.signal,
    });

  let token = sessionStorage.getItem('access_token') || '';
  let response = await makeRequest(token);

  // If token expired, try to refresh and retry once
  if (await isTokenErrorResponse(response)) {
    const newToken = await doRefreshToken();
    if (newToken) {
      response = await makeRequest(newToken);
    }
  }

  return { response, abortController };
}

// 8. Stop generation
export async function stopGeneration(
  conversationId: number
): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>(`/chat/conversations/${conversationId}/stop`);
  return res.data;
}

// ============ SSE Stream Parser ============

export function parseSSEStream(
  response: Response,
  callbacks: {
    onToken?: (content: string) => void;
    onReference?: (references: ReferenceData[]) => void;
    onTitle?: (title: string) => void;
    onDone?: (data?: ReferenceData[] | string) => void;
    onError?: (error: string) => void;
    onSearchStarted?: () => void;
    onSearchResults?: (results: SearchResultItem[], summary: string) => void;
    onSearchBusy?: (message: string) => void;
    onGenerationStarted?: (type: string) => void;
    onGenerationResult?: (type: string, content: string) => void;
  },
  abortController?: AbortController
): void {
  const reader = response.body?.getReader();
  if (!reader) {
    callbacks.onError?.('无法读取响应流');
    return;
  }

  const decoder = new TextDecoder();
  let buffer = '';
  let currentEventType = '';

  (async () => {
    try {
      console.log('Starting SSE stream parsing...');
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          console.log('Stream reader done');
          break;
        }

        const chunk = decoder.decode(value, { stream: true });
        console.log('Raw chunk:', chunk);
        buffer += chunk;
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          const trimmedLine = line.trim();

          // Skip empty lines
          if (!trimmedLine) {
            currentEventType = '';
            continue;
          }

          console.log('Processing line:', trimmedLine);

          // Parse event type - support both "event: token" and "event:token"
          if (trimmedLine.startsWith('event:')) {
            currentEventType = trimmedLine.slice(6).trim();
            console.log('Event type:', currentEventType);
            continue;
          }

          // Parse data - support both "data: {...}" and "data:{...}"
          if (trimmedLine.startsWith('data:')) {
            const dataStr = trimmedLine.slice(5).trim();
            console.log('Data string:', dataStr);
            try {
              const data: StreamEvent = JSON.parse(dataStr);
              console.log('Parsed SSE event:', data);

              // Handle different event types - use data.type first, then currentEventType
              const eventType = data.type || currentEventType;
              console.log('Handling event type:', eventType);

              switch (eventType) {
                case 'token':
                  console.log('Calling onToken with:', data.content);
                  callbacks.onToken?.(data.content);
                  break;
                case 'reference':
                  callbacks.onReference?.(data.data as ReferenceData[]);
                  break;
                case 'title':
                  console.log('Title received:', data.content);
                  callbacks.onTitle?.(data.content);
                  break;
                case 'done':
                  // 后端将引用附加到 done 事件的 data 字段（原子发送，消除竞态）
                  callbacks.onDone?.(Array.isArray(data.data) ? data.data as ReferenceData[] : data.content);
                  break;
                case 'error':
                  callbacks.onError?.(data.content);
                  break;
                case 'tool_call':
                case 'tool_result':
                  // 工具调用中间事件，不需要展示给用户，静默忽略
                  console.log('Tool event:', eventType, data.content);
                  break;
                case 'search_started':
                  callbacks.onSearchStarted?.();
                  break;
                case 'search_results': {
                  const searchData = data.data as { results?: SearchResultItem[]; summary?: string } | null;
                  if (searchData?.results) {
                    callbacks.onSearchResults?.(searchData.results, searchData.summary || '');
                  }
                  break;
                }
                case 'search_busy':
                  callbacks.onSearchBusy?.(data.content || '请等待当前搜索任务完成');
                  break;
                case 'generation_started':
                  console.log('[Generation] started:', data.data);
                  callbacks.onGenerationStarted?.((data.data as string) || 'note');
                  break;
                case 'generation_result': {
                  const genData = data.data as { type?: string; content?: string } | null;
                  if (genData?.content) {
                    callbacks.onGenerationResult?.(genData.type || 'note', genData.content);
                  }
                  break;
                }
                default:
                  console.log('Unknown event type:', eventType);
                  // 未知事件类型，不作为 token 显示
                  break;
              }
            } catch (e) {
              console.error('Failed to parse SSE data:', e, 'Raw line:', trimmedLine);
              // Try to handle plain text response
              if (dataStr && !dataStr.startsWith('{')) {
                callbacks.onToken?.(dataStr);
              }
            }
          }
        }
      }

      // Stream ended - if we haven't received a done event, call onDone
      console.log('Stream ended, calling onDone');
      callbacks.onDone?.('');
    } catch (error) {
      const isAborted = abortController?.signal.aborted ?? false;
      console.log('[parseSSEStream] catch 触发, isAborted:', isAborted, 'error:', error);
      if (isAborted) {
        // Stream was intentionally aborted (user clicked stop or switched conversation)
        // Call onDone to preserve the accumulated content
        console.log('[parseSSEStream] 检测到 abort，调用 onDone 保存已累积内容');
        callbacks.onDone?.('');
      } else {
        const rawMessage = error instanceof Error ? error.message : '流读取错误';
        console.log('[parseSSEStream] 非 abort 错误，调用 onError:', rawMessage);
        callbacks.onError?.(getChatErrorMessage(rawMessage));
      }
    }
  })();
}
