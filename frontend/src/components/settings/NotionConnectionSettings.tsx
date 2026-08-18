import { AlertCircle, BookOpen, CheckCircle2, Loader2, Unlink } from 'lucide-react';
import type { NotionBindingStatus } from '../../api/notion';
import Badge from '../ui/Badge';
import Button from '../ui/Button';

export interface NotionConnectionSettingsProps {
  status: NotionBindingStatus | null;
  loading: boolean;
  error: string | null;
  onConnect: () => void;
  onUnbind: () => void;
}

export default function NotionConnectionSettings({
  status,
  loading,
  error,
  onConnect,
  onUnbind,
}: NotionConnectionSettingsProps) {
  const connected = status?.bound === true && status?.status !== 'revoked';

  return (
    <div className="space-y-4">
      {error && (
        <div className="p-4 rounded-xl bg-error/5 border border-error/20 flex items-center gap-3" role="alert">
          <AlertCircle size={18} className="text-error flex-shrink-0" />
          <p className="text-sm text-error font-medium">{error}</p>
        </div>
      )}

      {loading ? (
        <div className="bg-bg-card rounded-xl border border-border-light p-5 flex items-center gap-3 text-text-muted">
          <Loader2 size={20} className="animate-spin text-accent" />
          <span className="text-sm">正在获取 Notion 连接状态...</span>
        </div>
      ) : connected ? (
        <div className="bg-bg-card rounded-xl border border-border-light p-5">
          <div className="flex items-center justify-between gap-4 mb-4">
            <div className="flex items-center gap-3 min-w-0">
              <div className="w-10 h-10 rounded-lg bg-success/10 flex items-center justify-center flex-shrink-0">
                <CheckCircle2 size={20} className="text-success" />
              </div>
              <div className="min-w-0">
                <h3 className="text-sm font-semibold text-text-primary">Notion 已连接</h3>
                <p className="text-xs text-text-muted truncate">
                  {status?.workspace_name || '已授权的 Notion 工作区'}
                </p>
              </div>
            </div>
            <Badge variant="success">已连接</Badge>
          </div>
          <div className="flex justify-end">
            <Button variant="ghost" size="sm" onClick={onUnbind}>
              <Unlink size={14} /> 解绑 Notion
            </Button>
          </div>
        </div>
      ) : (
        <div className="bg-bg-card rounded-xl border border-border-light p-5">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-10 h-10 rounded-lg bg-accent/10 flex items-center justify-center">
              <BookOpen size={20} className="text-accent" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-text-primary">连接 Notion</h3>
              <p className="text-xs text-text-muted">
                {status?.status === 'revoked'
                  ? 'Notion 授权已失效，请重新登录并授权。'
                  : '授权后即可导入你已授予访问权限的 Notion 页面。'}
              </p>
            </div>
          </div>
          <div className="flex justify-end">
            <Button size="sm" onClick={onConnect}>
              登录并授权 Notion
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
