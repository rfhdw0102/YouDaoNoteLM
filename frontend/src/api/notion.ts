import client from './client';
import type { ApiResponse } from './auth';

export type NotionPageItem = {
  id: string;
  title: string;
  url: string;
  last_edited_time: string;
  has_children: boolean;
};

export type NotionBindingStatus = {
  bound: boolean;
  status?: 'active' | 'revoked';
  workspace_id?: string;
  workspace_name?: string;
  workspace_icon?: string;
};

export type NotionTaskStatus =
  | 'pending'
  | 'running'
  | 'completed'
  | 'partial_failed'
  | 'failed'
  | 'cancelled';

export type NotionImportTask = {
  task_id: string;
  task_type: string;
  notebook_id: number;
  total_count: number;
  processed_count: number;
  success_count: number;
  fail_count: number;
  status: NotionTaskStatus;
  error_detail: string;
  created_at: number;
};

export async function startOAuth(): Promise<ApiResponse<{ authorize_url: string }>> {
  const res = await client.get<ApiResponse<{ authorize_url: string }>>('/notion/oauth/start');
  return res.data;
}

export async function getBinding(): Promise<ApiResponse<NotionBindingStatus>> {
  const res = await client.get<ApiResponse<NotionBindingStatus>>('/notion/bind');
  return res.data;
}

export async function unbind(): Promise<ApiResponse<null>> {
  const res = await client.delete<ApiResponse<null>>('/notion/bind');
  return res.data;
}

export async function listPages(params: {
  query?: string;
  cursor?: string;
  page_size?: number;
}): Promise<ApiResponse<{ list: NotionPageItem[]; next_cursor?: string; has_more: boolean }>> {
  const res = await client.get<ApiResponse<{ list: NotionPageItem[]; next_cursor?: string; has_more: boolean }>>(
    '/notion/pages',
    { params },
  );
  return res.data;
}

export async function importPagesBatch(
  notebookId: number,
  pageIds: string[],
): Promise<ApiResponse<{ task_id: string; source_ids: number[] }>> {
  const res = await client.post<ApiResponse<{ task_id: string; source_ids: number[] }>>('/notion/import/batch', {
    notebook_id: notebookId,
    page_ids: pageIds,
  });
  return res.data;
}

export async function getImportTask(taskId: string): Promise<ApiResponse<NotionImportTask>> {
  const res = await client.get<ApiResponse<NotionImportTask>>(`/notion/import/tasks/${taskId}`);
  return res.data;
}

export async function cancelImportTask(taskId: string): Promise<ApiResponse<null>> {
  const res = await client.delete<ApiResponse<null>>(`/notion/import/tasks/${taskId}`);
  return res.data;
}
