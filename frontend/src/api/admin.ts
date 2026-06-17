import client from './client';

// ===== Types =====

export interface AdminUser {
  id: number;
  username: string;
  email: string;
  nickname: string;
  avatar: string;
  role: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface SysConfig {
  id: string;
  config_group: string;
  config_key: string;
  config_value: string;
  description: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface ConfigStatus {
  groups: {
    group: string;
    count: number;
    enabled_count: number;
  }[];
}

export interface PageResponse<T> {
  list: T[];
  total: number;
  page: number;
  size: number;
}

// ===== User Management =====

export async function listUsers(params: {
  keyword?: string;
  page?: number;
  size?: number;
}): Promise<{ code: number; data: PageResponse<AdminUser>; message?: string }> {
  const res = await client.get('/admin/users', { params });
  return res.data;
}

export async function updateUserStatus(
  userId: number,
  enabled: boolean
): Promise<{ code: number; message?: string }> {
  const res = await client.put(`/admin/users/${userId}/status`, { enabled });
  return res.data;
}

// ===== Config Management =====

export async function getConfigStatus(): Promise<{
  code: number;
  data: ConfigStatus;
  message?: string;
}> {
  const res = await client.get('/admin/config/status');
  return res.data;
}

export async function getConfigs(group: string): Promise<{
  code: number;
  data: SysConfig[];
  message?: string;
}> {
  const res = await client.get(`/admin/config/${group}`);
  return res.data;
}

export async function addConfig(
  group: string,
  configKey: string,
  configValue: string,
  description?: string
): Promise<{ code: number; message?: string }> {
  const res = await client.post(`/admin/config/${group}`, {
    config_key: configKey,
    config_value: configValue,
    description,
  });
  return res.data;
}

export async function updateConfig(
  group: string,
  key: string,
  configValue: string,
  enabled?: boolean
): Promise<{ code: number; message?: string }> {
  // configValue 是 JSON 字符串，需要解析为对象后发送
  let parsedValue: any;
  try {
    parsedValue = JSON.parse(configValue);
  } catch {
    parsedValue = configValue;
  }

  const res = await client.put(`/admin/config/${group}/${key}`, {
    config_value: parsedValue,
    enabled,
  });
  return res.data;
}

export async function deleteConfig(
  group: string,
  key: string
): Promise<{ code: number; message?: string }> {
  const res = await client.delete(`/admin/config/${group}/${key}`);
  return res.data;
}

// 获取系统配置的完整信息（包含 provider 参数）
export async function getConfigWithProvider(group: string): Promise<{
  code: number;
  data: {
    configs: SysConfig[];
    providers: {
      provider: string;
      display_name: string;
      required_keys: string[] | null;
      optional_keys: string[] | null;
      key_labels?: Record<string, string>;
    }[];
  };
  message?: string;
}> {
  const res = await client.get(`/admin/config/${group}/detailed`);
  return res.data;
}
