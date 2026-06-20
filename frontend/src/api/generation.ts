import client from './client';

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
    { timeout: 300000 }
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

export async function exportGenerationFile(req: GenerationExportRequest): Promise<GenerationExportFile> {
  const res = await client.post<Blob>('/generations/export', req, {
    responseType: 'blob',
    timeout: 180000,
  });

  return {
    blob: res.data,
    filename: parseAttachmentFilename(res.headers['content-disposition']) || `${req.title || 'generation-export'}.pptx`,
    contentType: res.headers['content-type'] || 'application/octet-stream',
  };
}

export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
