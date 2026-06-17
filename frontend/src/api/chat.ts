import client, { doRefreshToken } from './client';
import type { ApiResponse } from './auth';

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
  type: 'token' | 'reference' | 'done' | 'error' | 'message' | 'title' | 'tool_call' | 'tool_result';
  content: string;
  data: ReferenceData[] | number | null;
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

// 7. Send message (streaming) - returns a ReadableStream
export async function sendMessage(
  conversationId: number,
  content: string,
  sourceIds?: number[]
): Promise<Response> {
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
      }),
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

  return response;
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
    onDone?: (content: string) => void;
    onError?: (error: string) => void;
  }
): AbortController {
  const abortController = new AbortController();

  const reader = response.body?.getReader();
  if (!reader) {
    callbacks.onError?.('无法读取响应流');
    return abortController;
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
                  callbacks.onDone?.(data.content);
                  break;
                case 'error':
                  callbacks.onError?.(data.content);
                  break;
                case 'tool_call':
                case 'tool_result':
                  // 工具调用中间事件，不需要展示给用户，静默忽略
                  console.log('Tool event:', eventType, data.content);
                  break;
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
      console.error('Stream parsing error:', error);
      if (abortController.signal.aborted) {
        // Stream was intentionally aborted (user clicked stop)
        // Call onDone to preserve the accumulated content
        callbacks.onDone?.('');
      } else {
        callbacks.onError?.(error instanceof Error ? error.message : '流读取错误');
      }
    }
  })();

  return abortController;
}
