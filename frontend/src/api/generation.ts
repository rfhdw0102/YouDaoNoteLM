import client from './client';

export type GenerationExportType = 'note' | 'mindmap' | 'quiz' | 'ppt';

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
    contentType: String(res.headers['content-type'] || 'application/octet-stream'),
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
