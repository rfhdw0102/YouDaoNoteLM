import { useEffect, useState } from 'react';
import { AlertCircle, ArrowLeft, Check, ExternalLink, Loader2, RefreshCw, Search, Square, SquareCheck } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import * as notionApi from '../../api/notion';
import type { NotionPageItem } from '../../api/notion';
import { cn } from '../../utils/cn';
import { dedupePageIds } from '../../utils/notionImport';
import Button from '../ui/Button';

const MAX_SELECTED_PAGES = 50;
const PAGE_SIZE = 50;

export type NotionImportPanelProps = {
  onImport: (pageIds: string[]) => Promise<void>;
  onBack: () => void;
};

function formatLastEdited(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '最近编辑时间未知';
  return `最后编辑于 ${date.toLocaleString('zh-CN')}`;
}

function safeNotionURL(value: string): string | null {
  try {
    const url = new URL(value);
    const hostname = url.hostname.toLowerCase();
    if (url.protocol === 'https:' && (hostname === 'notion.so' || hostname.endsWith('.notion.so'))) {
      return url.href;
    }
  } catch {
    // Do not render malformed provider URLs as navigable links.
  }
  return null;
}

export default function NotionImportPanel({ onImport, onBack }: NotionImportPanelProps) {
  const navigate = useNavigate();
  const [connection, setConnection] = useState<'checking' | 'connected' | 'disconnected'>('checking');
  const [pages, setPages] = useState<NotionPageItem[]>([]);
  const [query, setQuery] = useState('');
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [selectedPageIds, setSelectedPageIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadPages = async (append: boolean, search = query) => {
    if (append && (!hasMore || !nextCursor)) return;
    if (append) {
      setLoadingMore(true);
    } else {
      setLoading(true);
    }
    setError(null);
    try {
      const response = await notionApi.listPages({
        query: search.trim() || undefined,
        cursor: append ? nextCursor : undefined,
        page_size: PAGE_SIZE,
      });
      if (response.code !== 0) {
        setError(response.message || '加载 Notion 页面失败');
        return;
      }
      const incoming = response.data?.list ?? [];
      setPages((current) => append
        ? [...current, ...incoming.filter((page) => !current.some((existing) => existing.id === page.id))]
        : incoming);
      setNextCursor(response.data?.next_cursor);
      setHasMore(response.data?.has_more === true);
    } catch {
      setError('加载 Notion 页面失败，请稍后重试');
    } finally {
      if (append) {
        setLoadingMore(false);
      } else {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const response = await notionApi.getBinding();
        if (cancelled) return;
        if (response.code === 0 && response.data?.bound && response.data.status !== 'revoked') {
          setConnection('connected');
        } else {
          setConnection('disconnected');
        }
      } catch {
        if (!cancelled) setConnection('disconnected');
      }
    })();
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (connection !== 'connected') return;
    const timer = window.setTimeout(() => { void loadPages(false); }, 250);
    return () => window.clearTimeout(timer);
  // The page list intentionally reloads only when the search term or connection changes.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connection, query]);

  const togglePage = (pageId: string) => {
    setError(null);
    if (!selectedPageIds.has(pageId) && selectedPageIds.size >= MAX_SELECTED_PAGES) {
      setError(`一次最多导入 ${MAX_SELECTED_PAGES} 个 Notion 页面`);
      return;
    }
    setSelectedPageIds((current) => {
      const next = new Set(current);
      if (next.has(pageId)) {
        next.delete(pageId);
      } else {
        next.add(pageId);
      }
      return next;
    });
  };

  const handleImport = async () => {
    const pageIds = dedupePageIds([...selectedPageIds]);
    if (pageIds.length === 0) return;
    if (pageIds.length > MAX_SELECTED_PAGES) {
      setError(`一次最多导入 ${MAX_SELECTED_PAGES} 个 Notion 页面`);
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onImport(pageIds);
    } catch (err) {
      setError(err instanceof Error ? err.message : '导入 Notion 页面失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-border flex-shrink-0">
        <button onClick={onBack} aria-label="返回导入方式" className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer">
          <ArrowLeft size={16} />
        </button>
        <div className="flex-1">
          <h3 className="text-sm font-medium text-text-primary">导入 Notion 页面</h3>
          <p className="text-xs text-text-muted">选择你已授权访问的页面</p>
        </div>
        {connection === 'connected' && (
          <Button variant="ghost" size="sm" onClick={() => void loadPages(false)} disabled={loading} aria-label="刷新 Notion 页面">
            <RefreshCw size={14} className={cn(loading && 'animate-spin')} />
          </Button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto p-4">
        {connection === 'checking' ? (
          <div className="flex items-center justify-center py-12 text-text-muted"><Loader2 size={20} className="animate-spin text-accent" /><span className="ml-2 text-sm">正在检查 Notion 连接...</span></div>
        ) : connection === 'disconnected' ? (
          <div className="flex flex-col items-center justify-center py-12 gap-3 text-center">
            <AlertCircle size={24} className="text-text-muted" />
            <p className="text-sm text-text-primary">Notion 尚未连接</p>
            <p className="text-xs text-text-muted">请先登录并授权 Notion，再选择页面导入。</p>
            <Button size="sm" onClick={() => navigate('/settings?tab=notion')}>前往设置</Button>
          </div>
        ) : (
          <>
            <div className="relative mb-3">
              <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-text-muted" />
              <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索 Notion 页面" className="w-full rounded-lg border border-border-light bg-bg-card py-2 pl-9 pr-3 text-sm text-text-primary outline-none focus:border-accent" />
            </div>
            {error && <div role="alert" className="mb-3 flex items-center gap-2 rounded-lg border border-error/20 bg-error/5 px-3 py-2 text-xs text-error"><AlertCircle size={14} />{error}</div>}
            {loading ? (
              <div className="flex items-center justify-center py-12 text-text-muted"><Loader2 size={20} className="animate-spin text-accent" /><span className="ml-2 text-sm">正在加载页面...</span></div>
            ) : pages.length === 0 ? (
              <div className="py-12 text-center text-sm text-text-muted">没有可导入的 Notion 页面</div>
            ) : (
              <div className="space-y-2">
                {pages.map((page) => {
                  const selected = selectedPageIds.has(page.id);
                  const pageURL = safeNotionURL(page.url);
                  return <div key={page.id} onClick={() => togglePage(page.id)} className={cn('flex items-center gap-3 rounded-lg border px-3 py-2.5 transition-colors cursor-pointer', selected ? 'border-accent/30 bg-accent/5' : 'border-border-light hover:border-accent/20 hover:bg-bg-hover')}>
                    {selected ? <SquareCheck size={16} className="text-accent flex-shrink-0" /> : <Square size={16} className="text-text-muted flex-shrink-0" />}
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm text-text-primary">{page.title || '未命名 Notion 页面'}</p>
                      <p className="text-xs text-text-muted">{formatLastEdited(page.last_edited_time)}</p>
                    </div>
                    {pageURL && <a href={pageURL} target="_blank" rel="noopener noreferrer" aria-label={`在 Notion 打开 ${page.title || '页面'}`} onClick={(event) => event.stopPropagation()} className="p-1 text-text-muted hover:text-accent"><ExternalLink size={14} /></a>}
                  </div>;
                })}
                {hasMore && <Button variant="secondary" size="sm" className="w-full" onClick={() => void loadPages(true)} disabled={loadingMore}>{loadingMore ? <><Loader2 size={14} className="animate-spin" /> 加载中...</> : '加载更多'}</Button>}
              </div>
            )}
          </>
        )}
      </div>

      {connection === 'connected' && <div className="flex-shrink-0 border-t border-border p-4">
        <div className="mb-3 flex items-center justify-between text-xs text-text-muted"><span>已选 {selectedPageIds.size} 个页面</span><span>最多 {MAX_SELECTED_PAGES} 个</span></div>
        <Button className="w-full" onClick={() => void handleImport()} disabled={selectedPageIds.size === 0 || submitting}>{submitting ? <><Loader2 size={14} className="animate-spin" /> 导入中...</> : <><Check size={14} /> 导入选中页面</>}</Button>
      </div>}
    </div>
  );
}
