import client from './client';

export type MemoryType =
  | 'language'
  | 'answer_length'
  | 'answer_style'
  | 'output_format'
  | 'generation_style'
  | 'custom_instruction';

export interface UserMemory {
  type: MemoryType;
  content: string;
  updated_at: string;
}

interface ApiResponse<T> {
  code: number;
  message?: string;
  data: T;
}

export async function listUserMemories(): Promise<ApiResponse<UserMemory[]>> {
  const res = await client.get<ApiResponse<UserMemory[]>>('/user/memories');
  return res.data;
}

export async function upsertUserMemory(
  type: MemoryType,
  content: string,
): Promise<ApiResponse<UserMemory>> {
  const res = await client.put<ApiResponse<UserMemory>>(`/user/memories/${type}`, { content });
  return res.data;
}

export async function deleteUserMemory(type: MemoryType): Promise<ApiResponse<null>> {
  const res = await client.delete<ApiResponse<null>>(`/user/memories/${type}`);
  return res.data;
}
