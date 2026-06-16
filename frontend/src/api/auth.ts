import client from './client';

// Types
export interface CaptchaData {
  captcha_id: string;
  background: string;   // base64 data URL
  slider: string;       // base64 data URL
  slider_size: number;
  bg_width: number;     // background image actual width in pixels
  bg_height?: number;   // background image actual height in pixels
  slider_start_x: number; // slider initial X position
  slider_start_y?: number; // slider Y position (optional, defaults to center)
}

export interface UserData {
  id: number;
  username: string;
  email: string;
  nickname: string;
  avatar: string;
  role: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  user: UserData;
}

export interface ApiResponse<T = null> {
  code: number;
  message: string;
  data: T;
}

// 1. Get slider captcha
export async function getCaptcha(): Promise<ApiResponse<CaptchaData>> {
  const res = await client.get<ApiResponse<CaptchaData>>('/auth/captcha');
  return res.data;
}

// 2. Login
export async function login(params: {
  email: string;
  password: string;
  captcha_id: string;
  captcha_x: number;
}): Promise<ApiResponse<LoginResponse>> {
  const res = await client.post<ApiResponse<LoginResponse>>('/auth/login', params);
  return res.data;
}

// 3. Refresh token
export async function refreshToken(refresh_token: string): Promise<ApiResponse<LoginResponse>> {
  const res = await client.post<ApiResponse<LoginResponse>>('/auth/refresh', { refresh_token });
  return res.data;
}

// 4. Send verification code
export async function sendCode(params: {
  email: string;
  type: 'register' | 'reset' | 'delete_account';
}): Promise<ApiResponse<{ retry_after: number }>> {
  const res = await client.post<ApiResponse<{ retry_after: number }>>('/auth/send-code', params);
  return res.data;
}

// 5. Register
export async function register(params: {
  email: string;
  password: string;
  confirm_password: string;
  code: string;
}): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>('/auth/register', params);
  return res.data;
}

// 6. Reset password
export async function resetPassword(params: {
  email: string;
  code: string;
  new_password: string;
  confirm_password: string;
}): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>('/auth/reset-password', params);
  return res.data;
}

// 7. Logout - revoke tokens on server
export async function logout(params: {
  access_token?: string;
  refresh_token?: string;
}): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>('/auth/logout', params);
  return res.data;
}
