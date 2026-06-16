import client from './client';
import type { ApiResponse } from './auth';

export interface SearchResultItem {
  title: string;
  url: string;
  snippet: string;
  score: number;
  reason: string;
}

export interface SearchResponseData {
  results: SearchResultItem[];
  summary: string;
  search_rounds: number;
}

export interface TaskIDData {
  task_id: string;
}

export interface URLImportData {
  task_id: string;
  source_id: number;
}

export interface SearchImportData {
  task_id: string;
  source_ids: number[];
}

// 导入项（带标题）
export interface SearchImportItem {
  title: string;
  url: string;
}

// SSE 事件类型（与后端 SearchAgentEvent 对应）
export interface SearchStreamEvent {
  type: 'content' | 'tool_call' | 'search_round' | 'error' | 'done';
  content?: string;
  role?: string;
  tool_name?: string;
  tool_args?: string;
  search_rounds?: number;
  error?: string;
  error_code?: number; // 后端错误码，用于精确判断错误类型
}

// 1. Intelligent search (Agent-based) - 保留同步版本作为降级
export async function search(nbId: number, query: string): Promise<ApiResponse<SearchResponseData>> {
  const res = await client.post<ApiResponse<SearchResponseData>>(`/notebooks/${nbId}/search`, { query }, { timeout: 180000 }); // 3 minutes timeout
  return res.data;
}

// 1b. Intelligent search (SSE 流式)
export async function searchStream(
  nbId: number,
  query: string,
  onEvent: (event: SearchStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const token = sessionStorage.getItem('access_token');
  const res = await fetch(`/api/v1/notebooks/${nbId}/search/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ query }),
    signal,
  });

  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }

  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed.startsWith('data: ')) {
          let event: SearchStreamEvent;
          try {
            event = JSON.parse(trimmed.slice(6));
          } catch {
            // 忽略 JSON 解析错误
            continue;
          }
          // onEvent 可能会抛出异常（如错误事件），需要向上传播
          onEvent(event);
        }
      }
    }
  } finally {
    reader.releaseLock();
  }
}

// 2. Import from single URL
export async function importFromURL(nbId: number, url: string): Promise<ApiResponse<URLImportData>> {
  const res = await client.post<ApiResponse<URLImportData>>(`/notebooks/${nbId}/search/url`, { url }, { timeout: 180000 }); // 3 minutes timeout
  return res.data;
}

// 3. Batch import from search results (with titles)
export async function importSearchResults(nbId: number, items: SearchImportItem[]): Promise<ApiResponse<SearchImportData>> {
  const res = await client.post<ApiResponse<SearchImportData>>(`/notebooks/${nbId}/search/import`, { items }, { timeout: 180000 }); // 3 minutes timeout
  return res.data;
}
