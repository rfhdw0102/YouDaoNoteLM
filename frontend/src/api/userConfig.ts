import client from './client';

// ===== Types =====

export interface UserConfig {
  id: number;
  user_id: number;
  config_type: string;
  name: string;
  provider: string;
  api_key: string;
  api_url: string;
  model?: string;
  daily_quota?: number;
  dimensions?: number;
  extra_config?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface UserLLMConfig {
  id: number;
  user_id: number;
  name: string;
  provider: string;
  api_key: string;
  api_url: string;
  model: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface UserConfigRequest {
  name: string;
  provider: string;
  api_key: string;
  api_url: string;
  model?: string;
  daily_quota?: number;
  dimensions?: number;
  extra_config?: Record<string, any>;
  enabled?: boolean;
}

export interface HealthCheckResult {
  healthy: boolean;
  message: string;
  latency_ms: number;
  detail?: string;
}

// ===== LLM Config =====

export async function listLLMConfigs(): Promise<{
  code: number;
  data: UserLLMConfig[];
  message?: string;
}> {
  const res = await client.get('/user/config/llm');
  return res.data;
}

export async function createLLMConfig(
  data: UserConfigRequest
): Promise<{ code: number; data: UserLLMConfig; message?: string }> {
  const res = await client.post('/user/config/llm', data);
  return res.data;
}

export async function updateLLMConfig(
  id: number,
  data: UserConfigRequest
): Promise<{ code: number; data: UserLLMConfig; message?: string }> {
  const res = await client.put(`/user/config/llm/${id}`, data);
  return res.data;
}

export async function deleteLLMConfig(
  id: number
): Promise<{ code: number; message?: string }> {
  const res = await client.delete(`/user/config/llm/${id}`);
  return res.data;
}

// ===== Search Config =====

export async function listSearchConfigs(): Promise<{
  code: number;
  data: UserConfig[];
  message?: string;
}> {
  const res = await client.get('/user/config/search');
  return res.data;
}

export async function createSearchConfig(
  data: UserConfigRequest
): Promise<{ code: number; data: UserConfig; message?: string }> {
  const res = await client.post('/user/config/search', data);
  return res.data;
}

export async function updateSearchConfig(
  id: number,
  data: UserConfigRequest
): Promise<{ code: number; data: UserConfig; message?: string }> {
  const res = await client.put(`/user/config/search/${id}`, data);
  return res.data;
}

export async function deleteSearchConfig(
  id: number
): Promise<{ code: number; message?: string }> {
  const res = await client.delete(`/user/config/search/${id}`);
  return res.data;
}

// ===== ASR Config =====

export async function listASRConfigs(): Promise<{
  code: number;
  data: UserConfig[];
  message?: string;
}> {
  const res = await client.get('/user/config/asr');
  return res.data;
}

export async function createASRConfig(
  data: UserConfigRequest
): Promise<{ code: number; data: UserConfig; message?: string }> {
  const res = await client.post('/user/config/asr', data);
  return res.data;
}

export async function updateASRConfig(
  id: number,
  data: UserConfigRequest
): Promise<{ code: number; data: UserConfig; message?: string }> {
  const res = await client.put(`/user/config/asr/${id}`, data);
  return res.data;
}

export async function deleteASRConfig(
  id: number
): Promise<{ code: number; message?: string }> {
  const res = await client.delete(`/user/config/asr/${id}`);
  return res.data;
}

// ===== Embedding Config =====

export async function listEmbeddingConfigs(): Promise<{
  code: number;
  data: UserConfig[];
  message?: string;
}> {
  const res = await client.get('/user/config/embedding');
  return res.data;
}

export async function createEmbeddingConfig(
  data: UserConfigRequest
): Promise<{ code: number; data: UserConfig; message?: string }> {
  const res = await client.post('/user/config/embedding', data);
  return res.data;
}

export async function updateEmbeddingConfig(
  id: number,
  data: UserConfigRequest
): Promise<{ code: number; data: UserConfig; message?: string }> {
  const res = await client.put(`/user/config/embedding/${id}`, data);
  return res.data;
}

export async function deleteEmbeddingConfig(
  id: number
): Promise<{ code: number; message?: string }> {
  const res = await client.delete(`/user/config/embedding/${id}`);
  return res.data;
}

// ===== Health Check =====

export async function testConfig(
  configType: string,
  data: UserConfigRequest
): Promise<{ code: number; message?: string; data?: HealthCheckResult }> {
  const res = await client.post(`/user/config/${configType}/test`, data);
  return res.data;
}
