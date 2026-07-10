import { useState, useRef, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  Upload, Link, Globe, Search, FileText, File, Music,
  Check, MoreHorizontal, Trash2, Edit3,
  SquareCheck, Square, X, Loader2, ArrowLeft, Plus, AlertCircle,
  ChevronDown
} from 'lucide-react';
import { useNotebookStore } from '../../stores/useNotebookStore';
import { cn } from '../../utils/cn';
import { formatFileSize } from '../../utils/format';
import { getErrorMessage } from '../../utils/error';
import type { Source, SourceType } from '../../types';
import type { SearchResultItem, SearchStreamEvent } from '../../api/search';
import Modal from '../ui/Modal';
import Button from '../ui/Button';
import Input from '../ui/Input';
import YoudaoImportPanel from './YoudaoImportPanel';
import * as youdaoApi from '../../api/youdao';

const sourceIcons: Record<SourceType, typeof FileText> = {
  file: FileText, url: Link, audio: Music, youdao: File, search: Globe,
};

// 解析 Agent 返回的内容，提取搜索结果 JSON
function parseSearchContent(content: string): { results: SearchResultItem[]; summary: string } {
  // 尝试提取 ```json ... ``` 代码块
  const jsonMatch = content.match(/```json\s*([\s\S]*?)```/) || content.match(/```\s*([\s\S]*?)```/);
  const jsonStr = jsonMatch ? jsonMatch[1].trim() : content.trim();

  try {
    const parsed = JSON.parse(jsonStr);
    if (parsed.results && Array.isArray(parsed.results)) {
      return {
        results: parsed.results,
        summary: parsed.summary || '',
      };
    }
  } catch {
    // JSON 解析失败，返回空
  }
  return { results: [], summary: '' };
}

export default function SourcesPanel() {
  const {
    currentNotebookId, getCurrentNotebook, toggleSourceSelection,
    removeSource, batchRemoveSources, deleteFailedSources, renameSource,
    importFile, previewAudio, confirmAudio, searchSourcesStream, importFromURL, importSearchResults, fetchSourceContent, getSourceDownloadURL, fetchSources,
    reimportSelected
  } = useNotebookStore();
  const notebook = getCurrentNotebook();

  const [searchQuery, setSearchQuery] = useState('');
  const [showImportModal, setShowImportModal] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [contextMenuId, setContextMenuId] = useState<string | null>(null);
  const [viewingSource, setViewingSource] = useState<Source | null>(null);
  const [viewContent, setViewContent] = useState<{ markdown?: string }>({});
  const [viewLoading, setViewLoading] = useState(false);

  // Inline search state
  const [importInput, setImportInput] = useState('');
  const [searchResults, setSearchResults] = useState<SearchResultItem[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchSummary, setSearchSummary] = useState('');
  const [isSearchPanelOpen, setIsSearchPanelOpen] = useState(false);
  const [isSearchPanelCollapsed, setIsSearchPanelCollapsed] = useState(false);
  const [selectedResults, setSelectedResults] = useState<Set<number>>(new Set());
  const [searchProgress, setSearchProgress] = useState('');
  const abortControllerRef = useRef<AbortController | null>(null);

  // Audio preview state
  const [audioPreview, setAudioPreview] = useState<{ previewId: string; content: string; fileName: string } | null>(null);
  const [audioPreviewContent, setAudioPreviewContent] = useState('');
  const [audioTranscribing, setAudioTranscribing] = useState(false);
  // 已确认但 API 未完成的 previewId 集合（用于立即阻止点击和隐藏通知）
  const [confirmedPreviewIds, setConfirmedPreviewIds] = useState<Set<string>>(new Set());

  // Reimport state
  const [reimporting, setReimporting] = useState(false);
  const [reimportResult, setReimportResult] = useState<{ count: number; message: string } | null>(null);

  // Batch delete / delete failed state
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [deleteFailedLoading, setDeleteFailedLoading] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  // Import selected search results state
  const [importingSelected, setImportingSelected] = useState(false);

  // 折叠状态
  const [expandedVectorized, setExpandedVectorized] = useState(true);
  const [expandedUnvectorized, setExpandedUnvectorized] = useState(true);

  // 监听 store 中 audio source 状态变化，转写完成时自动更新预览面板
  useEffect(() => {
    if (!audioPreview || !audioTranscribing || !notebook) return;
    const source = notebook.sources.find(s => s.previewId === audioPreview.previewId);
    if (source && source.status === 'ready' && source.content) {
      setAudioPreview(prev => prev ? { ...prev, content: source.content! } : prev);
      setAudioPreviewContent(source.content);
      setAudioTranscribing(false);
    } else if (source && source.status === 'error') {
      setAudioTranscribing(false);
    }
  }, [notebook?.sources, audioPreview, audioTranscribing]);

  if (!notebook || !currentNotebookId) return null;

  const filteredSources = notebook.sources.filter((s) =>
    s.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  // 计算失效来源数量
  const failedCount = notebook.sources.filter(s => s.status === 'error').length;

  // 分别统计已入库和未入库的选中数量
  const vectorizedSources = notebook.sources.filter(s => s.vectorized);
  const unvectorizedSources = notebook.sources.filter(s => !s.vectorized);
  const selectedVectorizedCount = vectorizedSources.filter((s) => s.selected).length;
  const selectedUnvectorizedCount = unvectorizedSources.filter((s) => s.selected).length;

  // 检测转写完成但未确认的音频（排除已确认但 API 未完成的）
  const pendingReadyAudios = notebook.sources.filter(
    s => s.type === 'audio' && s.previewId && s.status === 'ready' && s.content && !confirmedPreviewIds.has(s.previewId)
  );
  const showAudioNotification = pendingReadyAudios.length > 0 && audioPreview == null;

  const handleSelectAllVectorized = () => {
    if (selectedVectorizedCount === vectorizedSources.length) {
      // 取消选中所有已入库
      vectorizedSources.forEach(s => {
        if (s.selected) toggleSourceSelection(currentNotebookId, s.id);
      });
    } else {
      // 选中所有已入库
      vectorizedSources.forEach(s => {
        if (!s.selected) toggleSourceSelection(currentNotebookId, s.id);
      });
    }
  };

  const handleSelectAllUnvectorized = () => {
    if (selectedUnvectorizedCount === unvectorizedSources.length) {
      // 取消选中所有未入库
      unvectorizedSources.forEach(s => {
        if (s.selected) toggleSourceSelection(currentNotebookId, s.id);
      });
    } else {
      // 选中所有未入库
      unvectorizedSources.forEach(s => {
        if (!s.selected) toggleSourceSelection(currentNotebookId, s.id);
      });
    }
  };

  const handleStartRename = (id: string, name: string) => {
    setEditingId(id); setEditName(name); setContextMenuId(null);
  };

  const handleFinishRename = () => {
    if (editingId && editName.trim() && currentNotebookId) renameSource(currentNotebookId, editingId, editName.trim());
    setEditingId(null);
  };

  const handleBatchDelete = async () => {
    const selectedIds = notebook.sources.filter(s => s.selected).map(s => s.id);
    if (selectedIds.length === 0 || !currentNotebookId) return;

    setBatchDeleting(true);
    setDeleteError(null);
    try {
      // 先删除已存在的 sources
      await batchRemoveSources(currentNotebookId, selectedIds);
    } catch (err) {
      console.error('Batch delete failed:', err);
      setDeleteError('删除失败，请重试');
      // 3秒后自动清除错误提示
      setTimeout(() => setDeleteError(null), 3000);
    } finally {
      setBatchDeleting(false);
    }
  };

  const handleDeleteFailed = async () => {
    if (!currentNotebookId) return;

    setDeleteFailedLoading(true);
    setDeleteError(null);
    try {
      const count = await deleteFailedSources(currentNotebookId);
      if (count > 0) {
        console.log(`Deleted ${count} failed sources`);
      }
    } catch (err) {
      console.error('Delete failed sources failed:', err);
      setDeleteError('清除失效资料失败，请重试');
      setTimeout(() => setDeleteError(null), 3000);
    } finally {
      setDeleteFailedLoading(false);
    }
  };

  const handleReimportSelected = async () => {
    if (!currentNotebookId) return;

    // 获取选中的未入库资料 ID
    const selectedIds = unvectorizedSources.filter(s => s.selected).map(s => s.id);
    if (selectedIds.length === 0) return;

    setReimporting(true);
    setReimportResult(null);
    try {
      const count = await reimportSelected(selectedIds);
      setReimportResult({
        count,
        message: count > 0 ? `已开始重新导入 ${count} 份资料` : '导入失败，请检查向量模型配置',
      });
      // 3秒后自动清除结果提示
      setTimeout(() => setReimportResult(null), 3000);
    } catch (err) {
      console.error('Reimport selected failed:', err);
      setReimportResult({ count: 0, message: '重新导入失败，请检查向量模型配置' });
      setTimeout(() => setReimportResult(null), 3000);
    } finally {
      setReimporting(false);
    }
  };

  const isUrl = (text: string) => /^https?:\/\/\S+$/.test(text.trim());

  const handleImportInputKeyDown = async (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && importInput.trim()) {
      if (isUrl(importInput)) {
        // URL import via API
        try {
          await importFromURL(currentNotebookId, importInput.trim());
          setImportInput('');
        } catch (err) {
          console.error('URL import failed:', err);
        }
      } else {
        // Intelligent search via SSE stream
        const controller = new AbortController();
        abortControllerRef.current = controller;

        setIsSearching(true);
        setIsSearchPanelOpen(true);
        setIsSearchPanelCollapsed(false);
        setSearchResults([]);
        setSearchSummary('');
        setSearchProgress('正在连接...');

        let finalSummary = '';
        let finalResults: SearchResultItem[] = [];
        let finalRounds = 0;

        try {
          await searchSourcesStream(
            currentNotebookId,
            importInput.trim(),
            (event: SearchStreamEvent) => {
              switch (event.type) {
                case 'search_round':
                  finalRounds = event.search_rounds || 0;
                  setSearchProgress(`正在搜索... 第 ${finalRounds} 轮`);
                  break;
                case 'tool_call':
                  if (event.tool_name === 'web_search') {
                    setSearchProgress(`正在搜索网页...`);
                  } else if (event.tool_name === 'analyze_results') {
                    setSearchProgress('正在分析结果...');
                  } else if (event.tool_name === 'refine_query') {
                    setSearchProgress('正在优化关键词...');
                  }
                  break;
                case 'content':
                  // 解析最终内容中的 JSON 结果
                  if (event.content) {
                    const parsed = parseSearchContent(event.content);
                    if (parsed.results.length > 0) {
                      finalResults = parsed.results;
                    }
                    if (parsed.summary) {
                      finalSummary = parsed.summary;
                    }
                  }
                  break;
                case 'error': {
                  // 创建携带错误码的错误对象
                  const error: any = new Error(event.error || '搜索失败');
                  error.code = event.error_code;
                  throw error;
                }
                case 'done':
                  setSearchProgress('');
                  break;
              }
            },
            controller.signal,
          );

          setSearchResults(finalResults);
          setSearchSummary(finalSummary || '搜索完成');
        } catch (err: any) {
          if (err.name === 'AbortError') return;
          console.error('Search failed:', err);
          setSearchResults([]);

          // 优先使用错误码判断，回退到字符串匹配
          const errorCode = err.code;
          const msg = getErrorMessage(err, '未知错误');

          if (errorCode === 40010) {
            // CodeLLMNotConfigured
            setSearchSummary('搜索需要先配置 LLM 服务。请前往 设置 → AI 服务配置 添加 LLM 配置后再试。');
          } else if (msg.includes('LLM') || msg.includes('llm') || msg.includes('配置')) {
            setSearchSummary('搜索需要先配置 LLM 服务。请前往 设置 → AI 服务配置 添加 LLM 配置后再试。');
          } else {
            setSearchSummary(`搜索失败：${msg}`);
          }
        } finally {
          setIsSearching(false);
          setSearchProgress('');
          abortControllerRef.current = null;
        }
      }
    }
  };

  const handleSelectAllSearchResults = () => {
    if (selectedResults.size === searchResults.length) {
      setSelectedResults(new Set());
    } else {
      setSelectedResults(new Set(searchResults.map((_, i) => i)));
    }
  };

  const handleToggleResult = (index: number) => {
    const next = new Set(selectedResults);
    if (next.has(index)) {
      next.delete(index);
    } else {
      next.add(index);
    }
    setSelectedResults(next);
  };

  const handleImportSelected = async () => {
    const items = searchResults
      .filter((_, i) => selectedResults.has(i))
      .map(r => ({ title: r.title, url: r.url }));
    if (items.length === 0 || !currentNotebookId) return;

    setImportingSelected(true);
    try {
      await importSearchResults(currentNotebookId, items);
      // 导入成功后关闭搜索面板并清空输入
      handleCloseSearchPanel();
      setImportInput('');
    } catch (err) {
      console.error('Import search results failed:', err);
    } finally {
      setImportingSelected(false);
    }
  };

  const handleCloseSearchPanel = () => {
    // 取消正在进行的搜索请求
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
    setIsSearchPanelOpen(false);
    setIsSearching(false);
    setSearchResults([]);
    setSearchSummary('');
    setSearchProgress('');
    setSelectedResults(new Set());
  };

  const handleCollapseSearchPanel = () => {
    setIsSearchPanelCollapsed(true);
  };

  const handleExpandSearchPanel = () => {
    setIsSearchPanelCollapsed(false);
  };

  // Load source content for viewing
  const handleViewSource = async (source: Source) => {
    // loading 状态的 source 还在后台处理中，禁止点击查看（避免拿到空/旧内容）
    if (source.status === 'loading') return;

    // Audio sources with pending preview: reopen editor with confirm button
    // Only allow preview editing if the source has a previewId (not yet confirmed)
    // Also skip if this previewId is already being confirmed (in confirmedPreviewIds)
    if (source.type === 'audio' && source.previewId && !confirmedPreviewIds.has(source.previewId)) {
      const content = source.content || '';
      setAudioPreview({ previewId: source.previewId, content, fileName: source.name });
      setAudioPreviewContent(content);
      setAudioTranscribing(false);
      return;
    }
    // 已确认的音频源（previewId 在 confirmedPreviewIds 中）不允许点击
    if (source.type === 'audio' && source.previewId && confirmedPreviewIds.has(source.previewId)) {
      return;
    }
    // For confirmed audio sources (no previewId), view as read-only
    setViewingSource(source);
    setViewContent({});
    setViewLoading(true);
    try {
      // 统一从后端获取最新内容（避免本地缓存的旧内容未经结构化）
      const mdContent = await fetchSourceContent(currentNotebookId, source.id).catch(() => undefined);
      setViewContent({ markdown: mdContent });
    } catch {
      // ignore
    } finally {
      setViewLoading(false);
    }
  };

  // Download original file
  const handleDownloadFile = async (source: Source) => {
    try {
      const url = await getSourceDownloadURL(currentNotebookId, source.id);
      window.open(url, '_blank');
    } catch (err) {
      console.error('Download failed:', err);
    }
  };

  // ---- Audio Preview ----
  if (audioPreview) {
    return (
      <div className="h-full flex flex-col bg-bg-secondary/30">
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border flex-shrink-0">
          <button onClick={() => { setAudioPreview(null); setAudioTranscribing(false); }} className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer">
            <ArrowLeft size={16} />
          </button>
          <span className="text-sm font-medium text-text-primary truncate flex-1">音频转写预览 - {audioPreview.fileName}</span>
        </div>
        <div className="flex-1 overflow-y-auto p-4">
          {audioTranscribing ? (
            <div className="flex flex-col items-center justify-center h-full gap-3 text-text-muted">
              <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
              <p className="text-sm">音频转写中，请稍候...</p>
              <p className="text-xs">转写完成后将自动显示内容</p>
            </div>
          ) : (
            <div className="mb-3">
              <p className="text-xs text-text-muted mb-2">转写内容（可编辑修改后确认导入）：</p>
              <textarea
                value={audioPreviewContent}
                onChange={(e) => setAudioPreviewContent(e.target.value)}
                className="w-full h-64 p-3 rounded-lg bg-bg-card border border-border-light text-xs text-text-primary leading-relaxed resize-none focus:border-accent focus:outline-none"
              />
            </div>
          )}
        </div>
        <div className="flex-shrink-0 p-4 border-t border-border flex gap-2">
          <Button size="sm" variant="secondary" onClick={() => { setAudioPreview(null); setAudioTranscribing(false); }}>取消</Button>
          {audioPreview.previewId && !audioTranscribing && (
            <Button
              size="sm"
              onClick={async () => {
                if (!currentNotebookId) return;
                const previewId = audioPreview.previewId;
                const content = audioPreviewContent;
                // 立即将 previewId 加入已确认集合（阻止点击和隐藏通知）
                setConfirmedPreviewIds(prev => new Set(prev).add(previewId));
                // 立即退出预览界面
                setAudioPreview(null);
                setAudioTranscribing(false);
                // 后台执行确认导入
                try {
                  await confirmAudio(previewId, currentNotebookId, content);
                } catch (err) {
                  console.error('Confirm audio failed:', err);
                }
              }}
            >
              确认导入
            </Button>
          )}
        </div>
      </div>
    );
  }

  // ---- Content Viewer ----
  if (viewingSource) {
    return (
      <div className="h-full flex flex-col bg-bg-secondary/30">
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border flex-shrink-0">
          <button onClick={() => setViewingSource(null)} className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer">
            <ArrowLeft size={16} />
          </button>
          <span className="text-sm font-medium text-text-primary truncate flex-1">{viewingSource.name}</span>
        </div>
        {/* Download button for file types */}
        {viewingSource.type === 'file' && (
          <div className="px-4 py-2 border-b border-border flex-shrink-0">
            <button
              onClick={() => handleDownloadFile(viewingSource)}
              className="flex items-center gap-1.5 text-xs text-accent hover:text-accent-light transition-colors cursor-pointer"
            >
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5 5 5-5M12 15V3" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              下载原始文件
            </button>
          </div>
        )}
        {/* Visit original URL for url types */}
        {viewingSource.type === 'url' && viewingSource.url && (
          <div className="px-4 py-2 border-b border-border flex-shrink-0">
            <a
              href={viewingSource.url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 text-xs text-accent hover:text-accent-light transition-colors"
            >
              <Globe size={13} />
              访问原网站
            </a>
          </div>
        )}
        <div className="flex-1 overflow-y-auto p-4">
          <div className="bg-bg-card rounded-lg border border-border-light p-4">
            {viewLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 size={20} className="animate-spin text-accent" />
                <span className="ml-2 text-sm text-text-muted">加载中...</span>
              </div>
            ) : (
              <pre className="whitespace-pre-wrap text-xs text-text-secondary leading-relaxed font-[family-name:var(--font-mono)]">
                {viewContent.markdown || '暂无内容'}
              </pre>
            )}
          </div>
        </div>
      </div>
    );
  }

  // ---- Main Panel ----
  return (
    <div className="h-full flex flex-col bg-bg-secondary/30">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border flex-shrink-0">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-text-primary">资料来源</h3>
          <span className="text-xs text-text-muted bg-bg-hover px-1.5 py-0.5 rounded">{notebook.sources.length}</span>
        </div>
        <button onClick={() => setShowImportModal(true)} className="p-1.5 rounded-lg text-accent hover:bg-accent-glow transition-colors cursor-pointer">
          <Upload size={16} />
        </button>
      </div>

      {/* Search/URL input */}
      <div className="px-3 py-2 border-b border-border flex-shrink-0 space-y-2">
        <div className="relative">
          <Globe size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted" />
          <input type="text" placeholder="输入网址导入，或关键词智能搜索..." value={importInput} onChange={(e) => setImportInput(e.target.value)} onKeyDown={handleImportInputKeyDown}
            className="w-full h-8 pl-8 pr-3 rounded-md bg-bg-tertiary border border-border text-xs text-text-primary placeholder:text-text-muted focus:border-accent focus:outline-none" />
          {importInput && <button onClick={() => setImportInput('')} className="absolute right-2 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary cursor-pointer"><X size={12} /></button>}
        </div>

        {/* Search results dropdown */}
        {isSearchPanelOpen && (
          <div className="w-full bg-bg-card border border-border-light rounded-lg shadow-lg overflow-hidden">
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-secondary/50">
              <div className="flex items-center gap-2">
                {isSearchPanelCollapsed ? (
                  <button
                    onClick={handleExpandSearchPanel}
                    className="p-1.5 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
                    title="展开面板"
                  >
                    <Search size={16} />
                  </button>
                ) : (
                  <button
                    onClick={handleCollapseSearchPanel}
                    className="p-1.5 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
                    title="收起面板"
                  >
                    <ArrowLeft size={16} />
                  </button>
                )}
                <span className="text-sm font-medium text-text-primary">
                  {isSearching ? '搜索中...' : `搜索结果 (${searchResults.length})`}
                </span>
              </div>
              <button
                onClick={handleCloseSearchPanel}
                className="p-1.5 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
                title="关闭搜索"
              >
                <X size={16} />
              </button>
            </div>

            {/* Content - only show when not collapsed */}
            {!isSearchPanelCollapsed && (
              <div className="max-h-96 overflow-y-auto">
                {isSearching ? (
                  <div className="flex flex-col items-center justify-center py-8">
                    <Loader2 size={24} className="animate-spin text-accent mb-3" />
                    <span className="text-sm text-text-muted">AI 正在搜索和分析...</span>
                    {searchProgress && (
                      <span className="text-xs text-accent mt-1">{searchProgress}</span>
                    )}
                  </div>
                ) : (
                  <div className="p-3">
                    {/* Search summary */}
                    {searchSummary && (
                      <div className="mb-3 p-3 rounded-lg bg-accent/5 border border-accent/20">
                        <p className="text-xs text-text-secondary leading-relaxed">{searchSummary}</p>
                      </div>
                    )}

                    {/* Select all */}
                    {searchResults.length > 0 && (
                      <div className="flex items-center justify-between mb-2 px-1">
                        <button
                          onClick={handleSelectAllSearchResults}
                          className="text-xs text-accent hover:text-accent-light cursor-pointer"
                        >
                          {selectedResults.size === searchResults.length ? '取消全选' : '全选'}
                        </button>
                        <span className="text-xs text-text-muted">
                          已选择 {selectedResults.size} 项
                        </span>
                      </div>
                    )}

                    {/* Results list */}
                    <div className="space-y-2">
                      {searchResults.map((result, i) => (
                        <motion.div
                          key={i}
                          initial={{ opacity: 0, y: 4 }}
                          animate={{ opacity: 1, y: 0 }}
                          transition={{ delay: i * 0.03 }}
                          className={cn(
                            'flex items-center gap-3 p-3 rounded-lg cursor-pointer transition-all border',
                            selectedResults.has(i)
                              ? 'bg-accent/5 border-accent/20'
                              : 'hover:bg-bg-hover border-transparent'
                          )}
                          onClick={() => handleToggleResult(i)}
                        >
                          <div
                            className={cn(
                              'w-4 h-4 rounded border-2 flex items-center justify-center flex-shrink-0 transition-all',
                              selectedResults.has(i)
                                ? 'bg-accent border-accent'
                                : 'border-border-light'
                            )}
                          >
                            {selectedResults.has(i) && <Check size={10} className="text-white" />}
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium text-text-primary truncate">{result.title}</p>
                          </div>
                          <a
                            href={result.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs text-accent hover:text-accent-light flex-shrink-0"
                            onClick={(e) => e.stopPropagation()}
                          >
                            访问
                          </a>
                        </motion.div>
                      ))}
                    </div>

                    {/* Empty state */}
                    {searchResults.length === 0 && !isSearching && (
                      <div className="flex flex-col items-center justify-center py-6 text-text-muted">
                        <p className="text-sm">{searchSummary || '未找到相关结果'}</p>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Footer with import button */}
            {!isSearchPanelCollapsed && !isSearching && selectedResults.size > 0 && (
              <div className="border-t border-border p-3 bg-bg-secondary/50">
                <Button
                  size="sm"
                  className="w-full text-sm"
                  onClick={handleImportSelected}
                  disabled={importingSelected}
                >
                  {importingSelected ? (
                    <>
                      <Loader2 size={14} className="animate-spin" /> 导入中...
                    </>
                  ) : (
                    <>
                      <Plus size={14} /> 导入选中 ({selectedResults.size})
                    </>
                  )}
                </Button>
              </div>
            )}
          </div>
        )}

        <div className="relative">
          <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted" />
          <input type="text" placeholder="搜索已有来源..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full h-7 pl-8 pr-3 rounded-md bg-bg-tertiary border border-border text-xs text-text-primary placeholder:text-text-muted focus:border-accent focus:outline-none" />
        </div>
        {notebook.sources.length > 0 && (
          <div className="flex items-center justify-end gap-2">
            {selectedUnvectorizedCount > 0 && (
              <button
                onClick={handleReimportSelected}
                disabled={reimporting}
                className="flex items-center gap-1 text-xs text-accent hover:text-accent-light transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {reimporting ? <Loader2 size={11} className="animate-spin" /> : <Upload size={11} />}
                {reimporting ? '导入中...' : `导入选中 (${selectedUnvectorizedCount})`}
              </button>
            )}
            {failedCount > 0 && (
              <button onClick={handleDeleteFailed} disabled={deleteFailedLoading} className="flex items-center gap-1 text-xs text-warning hover:text-warning/80 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed">
                {deleteFailedLoading ? <Loader2 size={11} className="animate-spin" /> : <AlertCircle size={11} />}
                {deleteFailedLoading ? '清除中...' : `清除失效 (${failedCount})`}
              </button>
            )}
            {(selectedVectorizedCount > 0 || selectedUnvectorizedCount > 0) && (
              <button onClick={handleBatchDelete} disabled={batchDeleting} className="flex items-center gap-1 text-xs text-error hover:text-error/80 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed">
                {batchDeleting ? <Loader2 size={11} className="animate-spin" /> : <Trash2 size={11} />}
                {batchDeleting ? '删除中...' : `删除选中 (${selectedVectorizedCount + selectedUnvectorizedCount})`}
              </button>
            )}
          </div>
        )}

        {/* Reimport result notification */}
        {reimportResult && (
          <div className={cn(
            'px-3 py-2 text-xs',
            reimportResult.count > 0 ? 'bg-success/5 text-success' : 'bg-warning/5 text-warning'
          )}>
            {reimportResult.message}
          </div>
        )}

        {/* Delete error notification */}
        {deleteError && (
          <div className="px-3 py-2 text-xs bg-error/5 text-error">
            {deleteError}
          </div>
        )}
      </div>

      {/* Audio transcription complete notification */}
      {showAudioNotification && (
        <div className="px-3 py-2 border-b border-accent/30 bg-accent/5 flex-shrink-0">
          <button
            onClick={() => handleViewSource(pendingReadyAudios[0])}
            className="flex items-center gap-2 w-full text-left cursor-pointer group"
          >
            <div className="w-5 h-5 rounded-full bg-accent/20 flex items-center justify-center flex-shrink-0">
              <Music size={11} className="text-accent" />
            </div>
            <span className="text-xs text-accent group-hover:text-accent-light">
              {pendingReadyAudios.length === 1
                ? `音频「${pendingReadyAudios[0].name}」转写完成，点击查看并确认导入`
                : `${pendingReadyAudios.length} 个音频转写完成，点击查看并确认导入`}
            </span>
          </button>
        </div>
      )}

      {/* Content area */}
      <div className="flex-1 overflow-y-auto">
        <motion.div key="sources" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="py-1">
          {/* 已入库资料 */}
          {filteredSources.filter(s => s.vectorized).length > 0 && (
            <div className="mb-2">
              <div className="px-3 py-1.5 flex items-center justify-between hover:bg-bg-hover">
                <button
                  onClick={() => setExpandedVectorized(!expandedVectorized)}
                  className="flex items-center gap-1.5 cursor-pointer"
                >
                  <ChevronDown size={12} className={cn('text-text-muted transition-transform', !expandedVectorized && '-rotate-90')} />
                  <div className="w-1.5 h-1.5 rounded-full bg-success" />
                  <span className="text-[10px] font-medium text-text-muted uppercase tracking-wider">已入库</span>
                  <span className="text-[10px] text-text-muted">({filteredSources.filter(s => s.vectorized).length})</span>
                </button>
                <button
                  onClick={(e) => { e.stopPropagation(); handleSelectAllVectorized(); }}
                  className="flex items-center gap-1 text-[10px] text-text-muted hover:text-text-primary transition-colors cursor-pointer"
                >
                  {selectedVectorizedCount === vectorizedSources.length ? <SquareCheck size={12} className="text-accent" /> : <Square size={12} />}
                  {selectedVectorizedCount === vectorizedSources.length ? '取消' : '全选'}
                </button>
              </div>
              {expandedVectorized && filteredSources.filter(s => s.vectorized).map((source) => {
                const Icon = sourceIcons[source.type] || FileText;
                const isLoading = source.status === 'loading';
                const isError = source.status === 'error';
                const isConfirmingAudio = source.type === 'audio' && source.previewId && confirmedPreviewIds.has(source.previewId);
                return (
                  <motion.div key={source.id} layout initial={{ opacity: 0, x: -10 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -10 }}
                    className={cn(
                      'group relative flex items-start gap-2.5 px-3 py-2.5 mx-1.5 rounded-lg hover:bg-bg-hover border transition-all',
                      isError ? 'border-error/30 bg-error/5' :
                      isConfirmingAudio ? 'border-accent/20 bg-accent/5' :
                      'border-transparent'
                    )}>

                    {/* Checkbox - 已入库的资料可以被选中 */}
                    <div
                      onClick={(e) => {
                        e.stopPropagation();
                        if (isError || isConfirmingAudio) return;
                        toggleSourceSelection(currentNotebookId, source.id);
                      }}
                      className={cn(
                        'mt-0.5 w-4 h-4 rounded border-2 flex items-center justify-center flex-shrink-0 transition-all',
                        (isError || isConfirmingAudio)
                          ? 'border-border-light opacity-40 cursor-default'
                          : cn('cursor-pointer', source.selected ? 'bg-accent border-accent' : 'border-border-light hover:border-accent/50')
                      )}
                    >
                      {source.selected && !isError && !isConfirmingAudio && <Check size={10} className="text-white" />}
                    </div>

                    {/* Main content */}
                    <div className={cn('flex-1 min-w-0', (isLoading || isError || isConfirmingAudio) ? 'cursor-default' : 'cursor-pointer')} onClick={() => { if (!isLoading && !isError && !isConfirmingAudio) handleViewSource(source); }}>
                      <div className="flex items-center gap-1.5">
                        <div className={cn('p-1 rounded', isError ? 'bg-error/10 text-error' : '', !isError && source.type === 'file' && 'bg-blue-500/10 text-blue-400', !isError && source.type === 'url' && 'bg-teal/10 text-teal', !isError && source.type === 'audio' && 'bg-purple-500/10 text-purple-400', !isError && source.type === 'search' && 'bg-orange-500/10 text-orange-400', !isError && source.type === 'youdao' && 'bg-green-500/10 text-green-400')}>
                          {(isLoading || isConfirmingAudio) ? <Loader2 size={12} className="animate-spin" /> : isError ? <AlertCircle size={12} /> : <Icon size={12} />}
                        </div>
                        {editingId === source.id ? (
                          <input autoFocus value={editName} onChange={(e) => setEditName(e.target.value)} onBlur={handleFinishRename}
                            onKeyDown={(e) => e.key === 'Enter' && handleFinishRename()} className="flex-1 bg-transparent text-xs outline-none border-b border-accent" onClick={(e) => e.stopPropagation()} />
                        ) : (
                          <p className={cn('text-xs font-medium truncate', isError ? 'text-error' : 'text-text-primary')}>{source.name}</p>
                        )}
                      </div>
                      <div className="flex items-center gap-2 mt-0.5 ml-5">
                        {(isLoading || isConfirmingAudio) && <span className="text-[10px] text-accent animate-pulse">导入中...</span>}
                        {isError && (
                          <span className="text-[10px] text-error cursor-help" title={source.errorMessage || '导入失败'}>
                            {source.errorMessage || '导入失败'}
                          </span>
                        )}
                        {!isLoading && !isError && !isConfirmingAudio && source.size !== undefined && <span className="text-[10px] text-text-muted">{formatFileSize(source.size)}</span>}
                        <span className="text-[10px] text-success">✓ 已入库</span>
                      </div>
                    </div>

                    {/* Actions */}
                    {(isLoading || isConfirmingAudio) ? (
                      <button onClick={(e) => { e.stopPropagation(); removeSource(currentNotebookId, source.id); }}
                        className="p-1 rounded text-error hover:bg-error/10 transition-all cursor-pointer" title="中止导入">
                        <X size={13} />
                      </button>
                    ) : (
                      <button onClick={(e) => { e.stopPropagation(); setContextMenuId(contextMenuId === source.id ? null : source.id); }}
                        className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-bg-active transition-all cursor-pointer">
                        <MoreHorizontal size={13} />
                      </button>
                    )}
                    {contextMenuId === source.id && (
                      <>
                        <div className="fixed inset-0 z-40" onClick={() => setContextMenuId(null)} />
                        <div className="absolute right-0 top-full mt-1 w-32 bg-bg-card border border-border-light rounded-lg shadow-xl z-50 py-1">
                          {!isError && !isLoading && !isConfirmingAudio && <button onClick={(e) => { e.stopPropagation(); handleStartRename(source.id, source.name); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-text-secondary hover:bg-bg-hover cursor-pointer"><Edit3 size={11} /> 重命名</button>}
                          <button onClick={(e) => { e.stopPropagation(); removeSource(currentNotebookId, source.id); setContextMenuId(null); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-error hover:bg-error/5 cursor-pointer"><Trash2 size={11} /> 删除</button>
                        </div>
                      </>
                    )}
                  </motion.div>
                );
              })}
            </div>
          )}

          {/* 未入库资料 */}
          {filteredSources.filter(s => !s.vectorized).length > 0 && (
            <div className="mt-2">
              <div className="px-3 py-1.5 flex items-center justify-between hover:bg-bg-hover">
                <button
                  onClick={() => setExpandedUnvectorized(!expandedUnvectorized)}
                  className="flex items-center gap-1.5 cursor-pointer"
                >
                  <ChevronDown size={12} className={cn('text-text-muted transition-transform', !expandedUnvectorized && '-rotate-90')} />
                  <div className="w-1.5 h-1.5 rounded-full bg-warning" />
                  <span className="text-[10px] font-medium text-text-muted uppercase tracking-wider">未入库</span>
                  <span className="text-[10px] text-text-muted">({filteredSources.filter(s => !s.vectorized).length})</span>
                </button>
                <button
                  onClick={(e) => { e.stopPropagation(); handleSelectAllUnvectorized(); }}
                  className="flex items-center gap-1 text-[10px] text-text-muted hover:text-text-primary transition-colors cursor-pointer"
                >
                  {selectedUnvectorizedCount === unvectorizedSources.length ? <SquareCheck size={12} className="text-accent" /> : <Square size={12} />}
                  {selectedUnvectorizedCount === unvectorizedSources.length ? '取消' : '全选'}
                </button>
              </div>
              {expandedUnvectorized && filteredSources.filter(s => !s.vectorized).map((source) => {
                const Icon = sourceIcons[source.type] || FileText;
                const isLoading = source.status === 'loading';
                const isError = source.status === 'error';
                const isConfirmingAudio = source.type === 'audio' && source.previewId && confirmedPreviewIds.has(source.previewId);
                return (
                  <motion.div key={source.id} layout initial={{ opacity: 0, x: -10 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -10 }}
                    className={cn(
                      'group relative flex items-start gap-2.5 px-3 py-2.5 mx-1.5 rounded-lg hover:bg-bg-hover border transition-all',
                      isError ? 'border-error/30 bg-error/5' :
                      isConfirmingAudio ? 'border-accent/20 bg-accent/5' :
                      'border-transparent'
                    )}>

                    {/* 未入库的资料也可以选中 */}
                    <div
                      onClick={(e) => {
                        e.stopPropagation();
                        if (isError || isConfirmingAudio) return;
                        toggleSourceSelection(currentNotebookId, source.id);
                      }}
                      className={cn(
                        'mt-0.5 w-4 h-4 rounded border-2 flex items-center justify-center flex-shrink-0 transition-all',
                        (isError || isConfirmingAudio)
                          ? 'border-border-light opacity-40 cursor-default'
                          : cn('cursor-pointer', source.selected ? 'bg-accent border-accent' : 'border-border-light hover:border-accent/50')
                      )}
                    >
                      {source.selected && !isError && !isConfirmingAudio && <Check size={10} className="text-white" />}
                      {isLoading && <Loader2 size={10} className="animate-spin text-accent" />}
                    </div>

                    {/* Main content */}
                    <div className={cn('flex-1 min-w-0', (isLoading || isError || isConfirmingAudio) ? 'cursor-default' : 'cursor-pointer')} onClick={() => { if (!isLoading && !isError && !isConfirmingAudio) handleViewSource(source); }}>
                      <div className="flex items-center gap-1.5">
                        <div className={cn('p-1 rounded', isError ? 'bg-error/10 text-error' : '', !isError && source.type === 'file' && 'bg-blue-500/10 text-blue-400', !isError && source.type === 'url' && 'bg-teal/10 text-teal', !isError && source.type === 'audio' && 'bg-purple-500/10 text-purple-400', !isError && source.type === 'search' && 'bg-orange-500/10 text-orange-400', !isError && source.type === 'youdao' && 'bg-green-500/10 text-green-400')}>
                          {(isLoading || isConfirmingAudio) ? <Loader2 size={12} className="animate-spin" /> : isError ? <AlertCircle size={12} /> : <Icon size={12} />}
                        </div>
                        {editingId === source.id ? (
                          <input autoFocus value={editName} onChange={(e) => setEditName(e.target.value)} onBlur={handleFinishRename}
                            onKeyDown={(e) => e.key === 'Enter' && handleFinishRename()} className="flex-1 bg-transparent text-xs outline-none border-b border-accent" onClick={(e) => e.stopPropagation()} />
                        ) : (
                          <p className={cn('text-xs font-medium truncate', isError ? 'text-error' : 'text-text-primary')}>{source.name}</p>
                        )}
                      </div>
                      <div className="flex items-center gap-2 mt-0.5 ml-5">
                        {(isLoading || isConfirmingAudio) && <span className="text-[10px] text-accent animate-pulse">导入中...</span>}
                        {isError && (
                          <span className="text-[10px] text-error cursor-help" title={source.errorMessage || '导入失败'}>
                            {source.errorMessage || '导入失败'}
                          </span>
                        )}
                        {!isLoading && !isError && !isConfirmingAudio && source.size !== undefined && <span className="text-[10px] text-text-muted">{formatFileSize(source.size)}</span>}
                        {!isLoading && !isError && <span className="text-[10px] text-warning">待入库</span>}
                      </div>
                    </div>

                    {/* Actions */}
                    {(isLoading || isConfirmingAudio) ? (
                      <button onClick={(e) => { e.stopPropagation(); removeSource(currentNotebookId, source.id); }}
                        className="p-1 rounded text-error hover:bg-error/10 transition-all cursor-pointer" title="中止导入">
                        <X size={13} />
                      </button>
                    ) : (
                      <div className="flex items-center gap-1">
                        <button onClick={(e) => { e.stopPropagation(); setContextMenuId(contextMenuId === source.id ? null : source.id); }}
                          className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-bg-active transition-all cursor-pointer">
                          <MoreHorizontal size={13} />
                        </button>
                      </div>
                    )}
                    {contextMenuId === source.id && (
                      <>
                        <div className="fixed inset-0 z-40" onClick={() => setContextMenuId(null)} />
                        <div className="absolute right-0 top-full mt-1 w-32 bg-bg-card border border-border-light rounded-lg shadow-xl z-50 py-1">
                          {!isError && !isLoading && <button onClick={(e) => { e.stopPropagation(); handleStartRename(source.id, source.name); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-text-secondary hover:bg-bg-hover cursor-pointer"><Edit3 size={11} /> 重命名</button>}
                          <button onClick={(e) => { e.stopPropagation(); removeSource(currentNotebookId, source.id); setContextMenuId(null); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-error hover:bg-error/5 cursor-pointer"><Trash2 size={11} /> 删除</button>
                        </div>
                      </>
                    )}
                  </motion.div>
                );
              })}
            </div>
          )}

          {/* Empty state */}
          {filteredSources.length === 0 && (
            <div className="flex flex-col items-center justify-center py-12 text-text-muted">
              <Upload size={32} className="mb-3 opacity-30" />
              <p className="text-xs">暂无资料来源</p>
              <button onClick={() => setShowImportModal(true)} className="mt-2 text-xs text-accent hover:text-accent-light cursor-pointer">导入资料</button>
            </div>
          )}
        </motion.div>
      </div>

      {/* Import Modal */}
      <Modal open={showImportModal} onClose={() => setShowImportModal(false)} title="导入资料" size="md">
        <ImportModalContent
          onFileImport={(file) => importFile(currentNotebookId, file).then(() => setShowImportModal(false)).catch(console.error)}
          onAudioImport={async (file) => {
            try {
              await previewAudio(currentNotebookId, file);
              setShowImportModal(false);
              // 不跳转到转写预览面板，转写完成后通过通知横幅提醒用户
            } catch (err) { console.error(err); }
          }}
          onUrlImport={async (url) => {
            try {
              await importFromURL(currentNotebookId, url);
              setShowImportModal(false);
            } catch (err) { console.error(err); }
          }}
          onYoudaoImport={async (fileIds, fileNames) => {
            try {
              const res = await youdaoApi.importNotesBatch(fileIds, Number(currentNotebookId), fileNames);
              if (res.code === 0) {
                // 导入任务已创建，刷新 sources 列表并开始轮询状态
                await fetchSources(currentNotebookId!);
                setShowImportModal(false);

                // 轮询这些 source 的状态，直到全部处理完成
                const sourceIds = res.data.source_ids;
                if (sourceIds.length > 0) {
                  const pollSources = async () => {
                    const maxAttempts = 120;
                    for (let i = 0; i < maxAttempts; i++) {
                      await new Promise(r => setTimeout(r, 2000));
                      try {
                        await fetchSources(currentNotebookId!);
                        const notebook = getCurrentNotebook();
                        if (!notebook) return;
                        const pendingCount = notebook.sources.filter(
                          s => sourceIds.includes(Number(s.id)) && (s.status === 'loading')
                        ).length;
                        if (pendingCount === 0) return;
                      } catch {
                        return;
                      }
                    }
                  };
                  pollSources();
                }
              } else {
                console.error('Import failed:', res.message);
              }
            } catch (err) { console.error(err); }
          }}
        />
      </Modal>
    </div>
  );
}

function ImportModalContent({ onFileImport, onAudioImport, onUrlImport, onYoudaoImport }: {
  onFileImport: (file: File) => Promise<void>;
  onAudioImport: (file: File) => void;
  onUrlImport: (url: string) => void;
  onYoudaoImport: (fileIds: string[], fileNames: Record<string, string>) => Promise<void>;
}) {
  const [tab, setTab] = useState<'youdao' | 'file' | 'url'>('youdao');
  const [urlValue, setUrlValue] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [showYoudaoPanel, setShowYoudaoPanel] = useState(false);

  const audioExts = ['.mp3', '.wav'];

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const ext = '.' + file.name.split('.').pop()?.toLowerCase();
      if (audioExts.includes(ext)) {
        await onAudioImport(file);
      } else {
        await onFileImport(file);
      }
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleUrlImport = () => {
    if (!urlValue.trim()) return;
    onUrlImport(urlValue.trim());
  };

  const handleYoudaoImport = async (fileIds: string[], fileNames: Record<string, string>) => {
    try {
      await onYoudaoImport(fileIds, fileNames);
    } catch (err) {
      console.error('Youdao import failed:', err);
    }
  };

  // 如果显示有道云面板，则渲染有道云导入面板
  if (showYoudaoPanel) {
    return (
      <YoudaoImportPanel
        onImport={handleYoudaoImport}
        onBack={() => setShowYoudaoPanel(false)}
      />
    );
  }

  return (
    <div>
      <div className="flex gap-1 mb-5 bg-bg-tertiary rounded-lg p-1">
        <button onClick={() => setTab('youdao')} className={cn('flex-1 flex items-center justify-center gap-1.5 py-2 rounded-md text-xs font-medium transition-all cursor-pointer', tab === 'youdao' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary')}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
            <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
          </svg>
          有道云
        </button>
        <button onClick={() => setTab('file')} className={cn('flex-1 flex items-center justify-center gap-1.5 py-2 rounded-md text-xs font-medium transition-all cursor-pointer', tab === 'file' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary')}><Upload size={13} /> 文件上传</button>
        <button onClick={() => setTab('url')} className={cn('flex-1 flex items-center justify-center gap-1.5 py-2 rounded-md text-xs font-medium transition-all cursor-pointer', tab === 'url' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary')}><Link size={13} /> 网址导入</button>
      </div>

      {tab === 'youdao' && (
        <div className="space-y-4">
          <div className="border-2 border-dashed border-border-light rounded-xl p-8 text-center hover:border-accent/40 transition-colors">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="mx-auto mb-3 text-text-muted">
              <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
              <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
              <path d="M8 7h8M8 11h6" />
            </svg>
            <p className="text-sm text-text-secondary mb-1">导入有道云笔记</p>
            <p className="text-xs text-text-muted">支持批量导入有道云笔记到当前笔记本</p>
            <div className="mt-4 space-y-2">
              <Button size="sm" className="w-full" onClick={() => setShowYoudaoPanel(true)}>
                浏览有道云笔记
              </Button>
              <p className="text-xs text-text-muted">
                请先在设置中绑定有道云账号
              </p>
            </div>
          </div>
        </div>
      )}

      {tab === 'file' && (
        <div className="border-2 border-dashed border-border-light rounded-xl p-8 text-center hover:border-accent/40 transition-colors">
          <Upload size={32} className="mx-auto mb-3 text-text-muted" />
          <p className="text-sm text-text-secondary mb-1">拖拽文件到此处，或点击选择</p>
          <p className="text-xs text-text-muted">支持 PDF, DOCX, TXT, MD, HTML</p>
          <p className="text-xs text-text-muted mt-1">音频支持 MP3, WAV</p>
          {uploading ? (
            <div className="mt-4 flex items-center justify-center gap-2">
              <Loader2 size={16} className="animate-spin text-accent" />
              <span className="text-sm text-text-muted">上传中...</span>
            </div>
          ) : (
            <>
              <Button size="sm" className="mt-4" onClick={() => fileInputRef.current?.click()}>选择文件</Button>
              <input ref={fileInputRef} type="file" accept=".pdf,.docx,.txt,.md,.html,.pptx,.mp3,.wav" className="hidden" onChange={handleFileSelect} />
            </>
          )}
        </div>
      )}

      {tab === 'url' && (
        <div className="space-y-4">
          <Input label="网页地址" placeholder="https://example.com/article" icon={<Link size={16} />} value={urlValue} onChange={(e) => setUrlValue(e.target.value)} onKeyDown={(e: React.KeyboardEvent) => e.key === 'Enter' && handleUrlImport()} />
          <p className="text-xs text-text-muted">系统将自动抓取网页正文内容并转为 Markdown 格式</p>
          <Button onClick={handleUrlImport} className="w-full" disabled={!urlValue.trim()}>导入</Button>
        </div>
      )}
    </div>
  );
}
