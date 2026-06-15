import client from './client';
import type { ApiResponse } from './auth';

export interface NotebookData {
  id: number;
  name: string;
  created_at: string;
  updated_at: string;
}

// 1. Create notebook
export async function createNotebook(name: string): Promise<ApiResponse<NotebookData>> {
  const res = await client.post<ApiResponse<NotebookData>>('/notebooks', { name });
  return res.data;
}

// 2. List all notebooks
export async function listNotebooks(): Promise<ApiResponse<NotebookData[]>> {
  const res = await client.get<ApiResponse<NotebookData[]>>('/notebooks');
  return res.data;
}

// 3. Rename notebook
export async function renameNotebook(id: number, name: string): Promise<ApiResponse> {
  const res = await client.put<ApiResponse>(`/notebooks/${id}`, { name });
  return res.data;
}

// 4. Delete notebook
export async function deleteNotebook(id: number): Promise<ApiResponse> {
  const res = await client.delete<ApiResponse>(`/notebooks/${id}`);
  return res.data;
}

// 5. Delete user account
export async function deleteAccount(params: {
  password: string;
  code: string;
}): Promise<ApiResponse<{ message: string }>> {
  const res = await client.delete<ApiResponse<{ message: string }>>('/user/account', { data: params });
  return res.data;
}
