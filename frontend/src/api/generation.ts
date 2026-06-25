import client, { doRefreshToken } from './client';

export type GenerationType = 'note' | 'mindmap' | 'quiz' | 'ppt';
export type GenerationExportType = GenerationType;

export interface GenerationExportRequest {
  type: GenerationExportType;
  content: string;
  title?: string;
  template?: string;
}

export interface GenerationExportFile {
  blob: Blob;
  filename: string;
  contentType: string;
}

// ============ 内容生成 API ============

export interface GenerationRequest {
  notebook_id: number;
  markdown: string;
  type: GenerationType;
  prompt?: string;
  options?: Record<string, unknown>;
  source_ids?: number[];
  use_web?: boolean;
  allow_degrade?: boolean;
}

export interface GenerationReference {
  source_id: number;
  source_name: string;
  content: string;
  score: number;
  heading?: string;
  chapter_path?: string;
}

export interface SearchResult {
  title: string;
  url: string;
  snippet: string;
  content?: string;
}

export interface GenerationResponse {
  type: GenerationType;
  content: string;
  references?: GenerationReference[];
  search_results?: SearchResult[];
  meta?: Record<string, unknown>;
}

export async function generateFromMarkdown(req: GenerationRequest): Promise<GenerationResponse> {
  const res = await client.post<{ code: number; data: GenerationResponse; message?: string }>(
    '/generations',
    req,
    { timeout: 900000 }
  );
  if (res.data.code !== 0) {
    throw new Error(res.data.message || '生成失败');
  }
  return res.data.data;
}

// ============ 导出 API ============

function parseAttachmentFilename(disposition?: string): string | null {
  if (!disposition) return null;

  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1].trim());
    } catch {
      return utf8Match[1].trim();
    }
  }

  const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
  return plainMatch?.[1]?.trim() || null;
}

async function readBusinessErrorFromBlob(blob: Blob): Promise<string> {
  if (!String(blob.type || '').includes('application/json')) return '';

  try {
    const body = JSON.parse(await blob.text()) as { code?: number; message?: string; error?: string };
    if (typeof body.code === 'number' && body.code !== 0) {
      return body.message || body.error || `导出失败: ${body.code}`;
    }
  } catch {
    return '';
  }
  return '';
}

function getHeaderValue(headers: unknown, key: string): string | undefined {
  if (!headers || typeof headers !== 'object') return undefined;
  const value = (headers as Record<string, unknown>)[key];
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return value.find((item): item is string => typeof item === 'string');
  return value != null ? String(value) : undefined;
}

export async function exportGenerationFile(req: GenerationExportRequest): Promise<GenerationExportFile> {
  const res = await client.post<Blob>('/generations/export', req, {
    responseType: 'blob',
    timeout: 180000,
  });

  // Check if the blob contains a token error (axios interceptor can't detect this for blob responses)
  const tokenError = await checkTokenError(res.data);
  if (tokenError) {
    if (tokenError.code === 1006) {
      // Try to refresh token and retry
      const newToken = await doRefreshToken();
      if (newToken) {
        const retryRes = await client.post<Blob>('/generations/export', req, {
          responseType: 'blob',
          timeout: 180000,
        });
        const retryBusinessError = await readBusinessErrorFromBlob(retryRes.data);
        if (retryBusinessError) {
          throw new Error(retryBusinessError);
        }
        return processExportResponse(retryRes, req);
      }
    }
    throw new Error(tokenError.message);
  }

  const businessError = await readBusinessErrorFromBlob(res.data);
  if (businessError) {
    throw new Error(businessError);
  }

  return processExportResponse(res, req);
}

async function checkTokenError(blob: Blob): Promise<{ code: number; message: string } | null> {
  if (!String(blob.type || '').includes('application/json')) return null;

  try {
    const body = JSON.parse(await blob.text()) as { code?: number; message?: string };
    if (body.code === 1005 || body.code === 1006) {
      return { code: body.code, message: body.message || 'token 已过期' };
    }
  } catch {
    return null;
  }
  return null;
}

function processExportResponse(
  res: { data: Blob; headers?: unknown },
  req: GenerationExportRequest
): GenerationExportFile {
  return {
    blob: res.data,
    filename:
      parseAttachmentFilename(getHeaderValue(res.headers, 'content-disposition')) ||
      `${req.title || 'generation-export'}.pptx`,
    contentType: getHeaderValue(res.headers, 'content-type') || 'application/octet-stream',
  };
}

export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  // 延迟撤销，确保浏览器有时间启动下载
  setTimeout(() => {
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }, 100);
}
