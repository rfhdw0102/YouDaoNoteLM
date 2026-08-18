import type { NotionTaskStatus } from '../api/notion';

export function dedupePageIds(ids: string[]): string[] {
  const uniqueIds = new Set<string>();

  for (const id of ids) {
    const pageId = id.trim();
    if (pageId) uniqueIds.add(pageId);
  }

  return [...uniqueIds];
}

export function isTerminalNotionTaskStatus(status: NotionTaskStatus): boolean {
  return status === 'completed'
    || status === 'partial_failed'
    || status === 'failed'
    || status === 'cancelled';
}

const oauthFailureMessages: Record<string, string> = {
  denied: '你已取消 Notion 授权。',
  invalid_state: 'Notion 授权链接已失效，请重新发起授权。',
  expired_state: 'Notion 授权链接已过期，请重新发起授权。',
  user_disabled: '当前账号不可用，无法连接 Notion。',
  exchange_failed: 'Notion 授权未完成，请稍后重试。',
  not_configured: 'Notion 暂未配置，请联系管理员。',
};

export function oauthMessage(result: string | null | undefined, reason?: string | null): string {
  if (result === 'success') return 'Notion 已连接，可以开始导入页面。';
  if (result === 'error') return oauthFailureMessages[reason ?? ''] ?? 'Notion 授权失败，请重新发起授权。';
  return 'Notion 授权结果无效，请重新发起授权。';
}
