import client from './client';
import type { ApiResponse } from './auth';
import type { UserData } from './auth';

// 7.5 Get current user profile
export async function getProfile(): Promise<ApiResponse<UserData>> {
  const res = await client.get<ApiResponse<UserData>>('/user/profile');
  return res.data;
}

// 7.6 Update user profile (nickname, avatar)
export async function updateProfile(params: {
  nickname?: string;
  avatar?: string;
}): Promise<ApiResponse> {
  const res = await client.put<ApiResponse>('/user/profile', params);
  return res.data;
}

// 8. Upload avatar
export async function uploadAvatar(file: File): Promise<ApiResponse<{ avatar: string }>> {
  const formData = new FormData();
  formData.append('avatar', file);
  const res = await client.post<ApiResponse<{ avatar: string }>>('/user/avatar', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  return res.data;
}

// 9. Change username
export async function changeUsername(username: string): Promise<ApiResponse> {
  const res = await client.put<ApiResponse>('/user/username', { username });
  return res.data;
}

// 10. Change password
export async function changePassword(params: {
  old_password: string;
  new_password: string;
}): Promise<ApiResponse<{ message: string }>> {
  const res = await client.post<ApiResponse<{ message: string }>>('/user/password', params);
  return res.data;
}
