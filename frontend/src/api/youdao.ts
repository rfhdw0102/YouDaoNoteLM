import client from './client';

// ===== Types =====

export interface YoudaoNoteItem {
  id: string;
  name: string;
  type: 'file' | 'dir';
  parentId: string;
}

export interface YoudaoBindStatus {
  bound: boolean;
  status?: string;
}

export interface YoudaoImportResult {
  id: number;
  created_at: string;
  updated_at: string;
  user_id: number;
  notebook_id: number;
  name: string;
  type: string;
  external_id: string;
  status: string;
  vectorized: boolean;
  markdown_content: string;
}

export interface YoudaoBatchImportResult {
  task_id: string;
  source_ids: number[];
}

// ===== API Functions =====

/**
 * 查询有道云笔记绑定状态
 */
export async function getBindStatus(): Promise<{
  code: number;
  data: YoudaoBindStatus;
  message?: string;
}> {
  const res = await client.get('/youdao/bind');
  return res.data;
}

/**
 * 绑定有道云笔记 API Key
 */
export async function bindApiKey(apiKey: string): Promise<{
  code: number;
  message?: string;
}> {
  const res = await client.post('/youdao/bind', { api_key: apiKey });
  return res.data;
}

/**
 * 解绑有道云笔记账号
 */
export async function unbind(): Promise<{
  code: number;
  message?: string;
}> {
  const res = await client.delete('/youdao/bind');
  return res.data;
}

/**
 * 浏览有道云笔记目录
 */
export async function listNotes(folderId?: string): Promise<{
  code: number;
  data: YoudaoNoteItem[];
  message?: string;
}> {
  const params = folderId ? { folderId } : {};
  const res = await client.get('/youdao/notes', { params });
  console.log('youdao listNotes response:', JSON.stringify(res.data, null, 2));
  return res.data;
}

/**
 * 导入单篇有道云笔记
 */
export async function importNote(fileId: string, notebookId: number): Promise<{
  code: number;
  data: YoudaoImportResult;
  message?: string;
}> {
  const res = await client.post('/youdao/import', {
    file_id: fileId,
    notebook_id: notebookId,
  });
  return res.data;
}

/**
 * 批量导入有道云笔记
 */
export async function importNotesBatch(fileIds: string[], notebookId: number, fileNames?: Record<string, string>): Promise<{
  code: number;
  data: YoudaoBatchImportResult;
  message?: string;
}> {
  const res = await client.post('/youdao/import/batch', {
    file_ids: fileIds,
    notebook_id: notebookId,
    file_names: fileNames,
  });
  return res.data;
}