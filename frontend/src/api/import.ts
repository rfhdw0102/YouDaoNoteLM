import client from './client';
import type { ApiResponse } from './auth';
import type { SourceData } from './source';

export interface AudioPreviewData {
  preview_id: string;
  file_name: string;
  status: 'pending' | 'processing' | 'ready' | 'failed' | 'confirmed';
}

export interface AudioPreviewStatusData {
  preview_id: string;
  user_id: number;
  notebook_id: number;
  file_name: string;
  file_path: string;
  file_size: number;
  transcribed_text: string;
  status: 'pending' | 'processing' | 'ready' | 'failed' | 'confirmed';
  error_msg: string;
  expires_at: number;
}

export interface ImportTaskData {
  task_id: string;
  task_type: string;
  total_count: number;
  processed_count: number;
  success_count: number;
  fail_count: number;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'partial_failed' | 'cancelled';
  error_detail: string;
}

export interface TaskIDData {
  task_id: string;
}

// 1. Import file
export async function importFile(nbId: number, file: File): Promise<ApiResponse<SourceData>> {
  const formData = new FormData();
  formData.append('file', file);
  const res = await client.post<ApiResponse<SourceData>>(`/notebooks/${nbId}/import/file`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  return res.data;
}

// 2. Audio preview (upload audio for transcription)
export async function previewAudio(nbId: number, file: File): Promise<ApiResponse<AudioPreviewData>> {
  const formData = new FormData();
  formData.append('file', file);
  const res = await client.post<ApiResponse<AudioPreviewData>>(`/notebooks/${nbId}/import/audio/preview`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  return res.data;
}

// 3. Confirm audio import
export async function getAudioPreviewStatus(previewId: string): Promise<ApiResponse<AudioPreviewStatusData>> {
  const res = await client.get<ApiResponse<AudioPreviewStatusData>>(`/import/audio/preview/${previewId}`);
  return res.data;
}

// 4. Confirm audio import
export async function confirmAudio(params: {
  preview_id: string;
  content?: string;
  notebook_id: number;
}): Promise<ApiResponse<SourceData>> {
  const res = await client.post<ApiResponse<SourceData>>('/import/audio/confirm', params);
  return res.data;
}

// 5. Query import task progress
export async function getImportTask(taskId: string): Promise<ApiResponse<ImportTaskData>> {
  const res = await client.get<ApiResponse<ImportTaskData>>(`/import/tasks/${taskId}`);
  return res.data;
}

// 6. Delete/Cancel import task
export async function deleteImportTask(taskId: string): Promise<ApiResponse> {
  const res = await client.delete<ApiResponse>(`/import/tasks/${taskId}`);
  return res.data;
}
