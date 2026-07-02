import client from './client';
import type { ApiResponse } from './auth';

export interface SourceData {
  id: number;
  notebook_id: number;
  name: string;
  type: 'file' | 'url' | 'audio' | 'note' | 'youdao';
  original_url: string;
  file_path: string;
  file_size: number;
  mime_type: string;
  status: 'pending' | 'processing' | 'ready' | 'failed';
  error_message: string;
  vectorized: boolean;
  created_at: string;
  updated_at: string;
}

export interface PageData<T> {
  list: T[];
  total: number;
  page: number;
  size: number;
  total_page: number;
}

// 1. List sources in a notebook (paginated)
export async function listSources(
  nbId: number,
  params?: { page?: number; size?: number; keyword?: string }
): Promise<ApiResponse<PageData<SourceData>>> {
  const res = await client.get<ApiResponse<PageData<SourceData>>>(`/notebooks/${nbId}/sources`, { params });
  return res.data;
}

// 2. Get source detail
export async function getSource(nbId: number, id: number): Promise<ApiResponse<SourceData>> {
  const res = await client.get<ApiResponse<SourceData>>(`/notebooks/${nbId}/sources/${id}`);
  return res.data;
}

// 3. Rename source
export async function renameSource(nbId: number, id: number, name: string): Promise<ApiResponse> {
  const res = await client.put<ApiResponse>(`/notebooks/${nbId}/sources/${id}`, { name });
  return res.data;
}

// 4. Delete source
export async function deleteSource(nbId: number, id: number): Promise<ApiResponse> {
  const res = await client.delete<ApiResponse>(`/notebooks/${nbId}/sources/${id}`);
  return res.data;
}

// 5. Batch delete sources
export async function batchDeleteSources(nbId: number, ids: number[]): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>(`/notebooks/${nbId}/sources/batch-delete`, { ids });
  return res.data;
}

// 5.1 Delete all failed sources
export async function deleteFailedSources(nbId: number): Promise<ApiResponse<{ deleted_count: number }>> {
  const res = await client.post<ApiResponse<{ deleted_count: number }>>(`/notebooks/${nbId}/sources/delete-failed`);
  return res.data;
}

// 6. Get source markdown content
export async function getSourceContent(nbId: number, id: number): Promise<ApiResponse<{ content: string }>> {
  const res = await client.get<ApiResponse<{ content: string }>>(`/notebooks/${nbId}/sources/${id}/content`, {
    params: { _t: Date.now() },
  });
  return res.data;
}

// 7. Get source original content
export async function getSourceOriginal(nbId: number, id: number): Promise<ApiResponse<{ content: string; type: string }>> {
  const res = await client.get<ApiResponse<{ content: string; type: string }>>(`/notebooks/${nbId}/sources/${id}/original`);
  return res.data;
}

// 8. Get file download URL (presigned MinIO URL)
export async function getSourceDownloadURL(nbId: number, id: number): Promise<ApiResponse<{ url: string }>> {
  const res = await client.get<ApiResponse<{ url: string }>>(`/notebooks/${nbId}/sources/${id}/download`);
  return res.data;
}

// 9. Reimport all unvectorized sources
export async function reimportAll(): Promise<ApiResponse<{ reimported_count: number }>> {
  const res = await client.post<ApiResponse<{ reimported_count: number }>>('/sources/reimport-all');
  return res.data;
}

// 10. Reimport specific sources by IDs
export async function reimportSources(sourceIds: number[]): Promise<ApiResponse<{ reimported_count: number }>> {
  const res = await client.post<ApiResponse<{ reimported_count: number }>>('/sources/reimport', { source_ids: sourceIds });
  return res.data;
}

// 11. Save note as source
export async function createSourceFromNote(nbId: number, title: string, content: string): Promise<ApiResponse<SourceData>> {
  const res = await client.post<ApiResponse<SourceData>>(`/notebooks/${nbId}/sources/from-note`, { title, content });
  return res.data;
}

// 12. Delete source by note title
export async function deleteSourceByNote(nbId: number, title: string): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>(`/notebooks/${nbId}/sources/delete-by-note`, { title });
  return res.data;
}
