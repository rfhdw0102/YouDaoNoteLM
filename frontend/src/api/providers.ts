import client from './client';

// ===== Types =====

export interface ProviderInfo {
  service_type: string;
  provider: string;
  display_name: string;
  required_keys: string[] | null;
  optional_keys: string[] | null;
  implemented: boolean;
  key_labels?: Record<string, string>;
}

export interface ProvidersResponse {
  code: number;
  data: {
    providers: ProviderInfo[];
  };
  message?: string;
}

// ===== API Functions =====

/**
 * 获取所有已注册的 Provider
 * @param serviceType 可选的服务类型过滤：search, llm, embedding, asr
 */
export async function listProviders(serviceType?: string): Promise<ProvidersResponse> {
  const params = serviceType ? { type: serviceType } : {};
  const res = await client.get('/providers', { params });
  return res.data;
}

/**
 * 按服务类型获取 Provider 列表
 */
export async function getProvidersByType(serviceType: string): Promise<ProviderInfo[]> {
  const res = await listProviders(serviceType);
  return res.data?.providers || [];
}

/**
 * 获取当前生效的配置（用户配置优先，否则系统配置）
 * @param serviceType 服务类型：search, llm, embedding, asr
 * @param userId 用户ID（可选，不传则返回系统配置）
 */
export async function getActiveConfig(serviceType: string, userId?: number): Promise<{
  code: number;
  data: {
    source: 'user' | 'system' | 'default';
    provider: string;
    display_name: string;
  };
  message?: string;
}> {
  const params: Record<string, any> = { type: serviceType };
  if (userId) params.user_id = userId;
  const res = await client.get('/providers/active', { params });
  return res.data;
}
