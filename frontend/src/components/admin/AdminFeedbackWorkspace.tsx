import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  BarChart3, List, Download, Loader2, AlertCircle,
  ThumbsUp, ThumbsDown, FileText, Calendar, Filter,
} from 'lucide-react';
import { cn } from '../../utils/cn';
import {
  getFeedbackOverview,
  listFeedback,
  exportFeedbackCSV,
  type FeedbackOverviewData,
  type AdminFeedbackItem,
} from '../../api/adminFeedback';

const REASON_LABELS: Record<string, string> = {
  preference_matched: '回答符合偏好',
  helpful: '回答有帮助',
  citation_reliable: '引用清晰可信',
  other: '其他',
  citation_inaccurate: '引用不准确',
  requirement_misunderstood: '记错要求',
  style_not_expected: '风格不符预期',
  not_helpful: '未解决问题',
};

const RATING_LABELS: Record<string, string> = { up: '点赞', down: '点踩' };

function getDefaultRange() {
  const now = new Date();
  return { from: new Date(now.getTime() - 7 * 864e5).toISOString(), to: now.toISOString() };
}

export default function AdminFeedbackWorkspace() {
  const [range, setRange] = useState(getDefaultRange);
  const [filterRating, setFilterRating] = useState('');
  const [filterReason, setFilterReason] = useState('');
  const [activeSection, setActiveSection] = useState<'overview' | 'list'>('overview');

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      className="space-y-5"
    >
      {/* Filters bar */}
      <div className="flex flex-wrap items-end gap-3 p-4 bg-bg-card border border-border-light rounded-2xl">
        <div className="flex items-center gap-2 text-text-muted mr-1">
          <Filter size={14} />
          <span className="text-xs font-medium">筛选</span>
        </div>
        <FilterField label="开始时间" icon={<Calendar size={11} />}>
          <input
            type="datetime-local"
            value={range.from.slice(0, 16)}
            onChange={(e) => setRange((r) => ({ ...r, from: new Date(e.target.value).toISOString() }))}
          />
        </FilterField>
        <FilterField label="结束时间" icon={<Calendar size={11} />}>
          <input
            type="datetime-local"
            value={range.to.slice(0, 16)}
            onChange={(e) => setRange((r) => ({ ...r, to: new Date(e.target.value).toISOString() }))}
          />
        </FilterField>
        <FilterField label="评价类型">
          <select value={filterRating} onChange={(e) => setFilterRating(e.target.value)}>
            <option value="">全部</option>
            <option value="up">点赞</option>
            <option value="down">点踩</option>
          </select>
        </FilterField>
        <FilterField label="原因">
          <select value={filterReason} onChange={(e) => setFilterReason(e.target.value)}>
            <option value="">全部</option>
            {Object.entries(REASON_LABELS).map(([c, l]) => <option key={c} value={c}>{l}</option>)}
          </select>
        </FilterField>
        <button
          onClick={() => setRange(getDefaultRange())}
          className="h-8 px-3 text-[11px] text-text-muted hover:text-accent border border-border-light rounded-lg hover:border-accent/30 transition-colors cursor-pointer"
        >
          最近 7 天
        </button>
      </div>

      {/* Section tabs */}
      <div className="flex gap-1 bg-bg-tertiary rounded-xl p-1 w-fit">
        {([['overview', '概览', BarChart3], ['list', '明细', List]] as const).map(([key, label, Icon]) => (
          <button
            key={key}
            onClick={() => setActiveSection(key)}
            className={cn(
              'flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium transition-all duration-200 cursor-pointer',
              activeSection === key
                ? 'bg-accent text-white shadow-sm shadow-accent/20'
                : 'text-text-muted hover:text-text-primary hover:bg-bg-hover'
            )}
          >
            <Icon size={13} /> {label}
          </button>
        ))}
      </div>

      {/* Content */}
      <AnimatePresence mode="wait">
        <motion.div
          key={activeSection}
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -6 }}
          transition={{ duration: 0.15 }}
        >
          {activeSection === 'overview'
            ? <OverviewSection range={range} filterRating={filterRating} filterReason={filterReason} />
            : <ListSection range={range} filterRating={filterRating} filterReason={filterReason} />
          }
        </motion.div>
      </AnimatePresence>
    </motion.div>
  );
}

function FilterField({ label, icon, children }: { label: string; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div>
      <label className="flex items-center gap-1 text-[10px] text-text-muted mb-1 uppercase tracking-wider">
        {icon} {label}
      </label>
      <div className="[&_input,&_select]:bg-bg-secondary [&_input,&_select]:border [&_input,&_select]:border-border-light [&_input,&_select]:rounded-lg [&_input,&_select]:px-2.5 [&_input,&_select]:py-1.5 [&_input,&_select]:text-xs [&_input,&_select]:text-text-primary [&_input,&_select]:outline-none [&_input,&_select]:focus:border-accent/50 [&_input,&_select]:transition-colors">
        {children}
      </div>
    </div>
  );
}

// ===== Overview =====

function OverviewSection({ range, filterRating, filterReason }: { range: { from: string; to: string }; filterRating: string; filterReason: string }) {
  const [data, setData] = useState<FeedbackOverviewData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true); setError(null);
      try {
        const params: Record<string, string> = { from: range.from, to: range.to };
        if (filterRating) params.rating = filterRating;
        if (filterReason) params.reason = filterReason;
        const res = await getFeedbackOverview(params as { from: string; to: string; rating?: string; reason?: string });
        if (!cancelled && res.code === 0) setData(res.data);
        else if (!cancelled) setError(res.message || '查询失败');
      } catch { if (!cancelled) setError('网络错误'); }
      finally { if (!cancelled) setLoading(false); }
    })();
    return () => { cancelled = true; };
  }, [range.from, range.to, filterRating, filterReason]);

  if (loading) return <Skeleton />;
  if (error) return <ErrorMsg error={error} />;
  if (!data) return null;

  const maxReason = Math.max(...Object.values(data.reason_counts), 1);

  return (
    <div className="space-y-4">
      {/* Stat cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="反馈总数" value={data.total_count} icon={<FileText size={15} />} />
        <StatCard label="点赞" value={data.up_count} icon={<ThumbsUp size={15} />} accent="accent" />
        <StatCard label="点踩" value={data.down_count} icon={<ThumbsDown size={15} />} accent="error" />
        <StatCard
          label="正向占比"
          value={data.total_count > 0 ? `${(data.positive_ratio * 100).toFixed(1)}%` : '—'}
          icon={<BarChart3 size={15} />}
          accent="accent"
        />
      </div>

      {/* Reason distribution */}
      <div className="bg-bg-card border border-border-light rounded-2xl p-5">
        <h3 className="text-xs font-medium text-text-secondary mb-4 uppercase tracking-wider">原因分布</h3>
        {Object.keys(data.reason_counts).length === 0 ? (
          <p className="text-sm text-text-muted text-center py-6">暂无数据</p>
        ) : (
          <div className="space-y-2.5">
            {Object.entries(data.reason_counts)
              .sort(([, a], [, b]) => b - a)
              .map(([reason, count], i) => {
                const pct = data.total_count > 0 ? (count / maxReason) * 100 : 0;
                return (
                  <motion.div
                    key={reason}
                    initial={{ opacity: 0, x: -8 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: i * 0.03 }}
                    className="flex items-center gap-3"
                  >
                    <span className="text-[11px] text-text-secondary w-28 truncate shrink-0">
                      {REASON_LABELS[reason] || reason}
                    </span>
                    <div className="flex-1 bg-bg-tertiary rounded-full h-2 overflow-hidden">
                      <motion.div
                        initial={{ width: 0 }}
                        animate={{ width: `${pct}%` }}
                        transition={{ duration: 0.5, ease: 'easeOut', delay: i * 0.03 }}
                        className="h-full bg-gradient-to-r from-accent/70 to-accent rounded-full"
                      />
                    </div>
                    <span className="text-[11px] text-text-muted w-10 text-right tabular-nums">{count}</span>
                  </motion.div>
                );
              })}
          </div>
        )}
      </div>

      <ExportButton range={range} filterRating={filterRating} filterReason={filterReason} />
    </div>
  );
}

function StatCard({ label, value, icon, accent }: { label: string; value: number | string; icon: React.ReactNode; accent?: string }) {
  const colorMap: Record<string, string> = { accent: 'text-accent', error: 'text-error' };
  const bgMap: Record<string, string> = { accent: 'bg-accent/10', error: 'bg-error/10' };
  return (
    <div className="bg-bg-card border border-border-light rounded-2xl p-4 group hover:border-border-active/30 transition-colors">
      <div className="flex items-center justify-between mb-3">
        <span className="text-[10px] text-text-muted uppercase tracking-wider">{label}</span>
        <div className={cn('w-7 h-7 rounded-xl flex items-center justify-center', accent ? bgMap[accent] : 'bg-bg-tertiary')}>
          <span className={cn(accent ? colorMap[accent] : 'text-text-muted')}>{icon}</span>
        </div>
      </div>
      <div className={cn('text-2xl font-bold tracking-tight', accent ? colorMap[accent] : 'text-text-primary')}>
        {value}
      </div>
    </div>
  );
}

// ===== List =====

function ListSection({ range, filterRating, filterReason }: { range: { from: string; to: string }; filterRating: string; filterReason: string }) {
  const [items, setItems] = useState<AdminFeedbackItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true); setError(null);
      try {
        const params: Record<string, string | number> = { from: range.from, to: range.to, page, size: 20 };
        if (filterRating) params.rating = filterRating;
        if (filterReason) params.reason = filterReason;
        const res = await listFeedback(params as { from: string; to: string; page: number; size: number; rating?: string; reason?: string });
        if (!cancelled && res.code === 0) { setItems(res.data.list); setTotal(res.data.total); }
        else if (!cancelled) setError(res.message || '查询失败');
      } catch { if (!cancelled) setError('网络错误'); }
      finally { if (!cancelled) setLoading(false); }
    })();
    return () => { cancelled = true; };
  }, [range.from, range.to, filterRating, filterReason, page]);

  const totalPages = Math.ceil(total / 20);

  if (loading) return <Skeleton />;
  if (error) return <ErrorMsg error={error} />;

  return (
    <div className="space-y-4">
      {items.length === 0 ? (
        <div className="text-center py-16 text-text-muted text-sm">暂无反馈数据</div>
      ) : (
        <div className="bg-bg-card border border-border-light rounded-2xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border/60">
                <th className="text-left px-5 py-3 text-[10px] text-text-muted font-medium uppercase tracking-wider">提交时间</th>
                <th className="text-left px-5 py-3 text-[10px] text-text-muted font-medium uppercase tracking-wider">评价</th>
                <th className="text-left px-5 py-3 text-[10px] text-text-muted font-medium uppercase tracking-wider">原因</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item, idx) => (
                <motion.tr
                  key={idx}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ delay: idx * 0.02 }}
                  className="border-b border-border/30 last:border-0 hover:bg-bg-hover/50 transition-colors"
                >
                  <td className="px-5 py-3 text-text-secondary text-xs tabular-nums">
                    {new Date(item.created_at).toLocaleString('zh-CN')}
                  </td>
                  <td className="px-5 py-3">
                    <span className={cn(
                      'inline-flex items-center gap-1.5 text-[11px] font-medium px-2.5 py-1 rounded-full',
                      item.rating === 'up' ? 'bg-accent/10 text-accent' : 'bg-error/10 text-error'
                    )}>
                      {item.rating === 'up' ? <ThumbsUp size={10} /> : <ThumbsDown size={10} />}
                      {RATING_LABELS[item.rating] || item.rating}
                    </span>
                  </td>
                  <td className="px-5 py-3 text-text-secondary text-xs">
                    {REASON_LABELS[item.reason] || item.reason}
                  </td>
                </motion.tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-xs text-text-muted">
          <span>共 {total} 条记录</span>
          <div className="flex items-center gap-1.5">
            <PageBtn disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>上一页</PageBtn>
            <span className="px-2 tabular-nums">{page} / {totalPages}</span>
            <PageBtn disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>下一页</PageBtn>
          </div>
        </div>
      )}

      <ExportButton range={range} filterRating={filterRating} filterReason={filterReason} />
    </div>
  );
}

function PageBtn({ disabled, onClick, children }: { disabled: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="h-7 px-3 text-[11px] rounded-lg border border-border-light hover:border-accent/30 hover:text-accent disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
    >
      {children}
    </button>
  );
}

// ===== Export =====

function ExportButton({ range, filterRating, filterReason }: { range: { from: string; to: string }; filterRating: string; filterReason: string }) {
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const handleExport = async () => {
    setExporting(true); setExportError(null);
    try {
      const params: Record<string, string> = { from: range.from, to: range.to };
      if (filterRating) params.rating = filterRating;
      if (filterReason) params.reason = filterReason;
      const response = await exportFeedbackCSV(params as { from: string; to: string; rating?: string; reason?: string });
      if (!response.ok) {
        try { setExportError((await response.json()).message || '导出失败'); }
        catch { setExportError('导出失败，请缩小范围'); }
        return;
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = response.headers.get('Content-Disposition')?.match(/filename="(.+)"/)?.[1] || 'feedback.csv';
      document.body.appendChild(a); a.click(); document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch { setExportError('网络错误'); }
    finally { setExporting(false); }
  };

  return (
    <div className="flex items-center gap-3">
      <button
        onClick={handleExport}
        disabled={exporting}
        className="flex items-center gap-2 h-9 px-4 text-xs font-medium bg-accent text-white rounded-xl hover:bg-accent-light shadow-sm shadow-accent/20 transition-all cursor-pointer disabled:opacity-50"
      >
        {exporting ? <Loader2 size={13} className="animate-spin" /> : <Download size={13} />}
        导出 CSV
      </button>
      {exportError && <span className="text-[11px] text-error">{exportError}</span>}
    </div>
  );
}

function Skeleton() {
  return (
    <div className="space-y-3 animate-pulse">
      <div className="grid grid-cols-4 gap-3">
        {[0, 1, 2, 3].map((i) => <div key={i} className="h-24 bg-bg-card border border-border-light rounded-2xl" />)}
      </div>
      <div className="h-48 bg-bg-card border border-border-light rounded-2xl" />
    </div>
  );
}

function ErrorMsg({ error }: { error: string }) {
  return (
    <div className="flex items-center gap-2 text-error text-sm py-6 justify-center">
      <AlertCircle size={14} /> {error}
    </div>
  );
}
