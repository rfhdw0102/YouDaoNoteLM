import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ArrowLeft, X, Check, Plus } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { SearchResultItem, SearchImportItem } from '../../api/search';
import Button from '../ui/Button';

interface SearchResultsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  onCollapse: () => void;
  searchResults: SearchResultItem[];
  searchSummary: string;
  onImportSelected: (items: SearchImportItem[]) => Promise<void>;
}

export default function SearchResultsPanel({
  isOpen,
  onClose,
  onCollapse,
  searchResults,
  searchSummary,
  onImportSelected
}: SearchResultsPanelProps) {
  const [selectedResults, setSelectedResults] = useState<Set<number>>(new Set());

  const handleSelectAll = () => {
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

  const handleImport = async () => {
    const items: SearchImportItem[] = searchResults
      .filter((_, i) => selectedResults.has(i))
      .map(r => ({ title: r.title, url: r.url }));
    if (items.length === 0) return;
    try {
      await onImportSelected(items);
      setSelectedResults(new Set());
    } catch (err) {
      console.error('Import search results failed:', err);
    }
  };

  const handleClose = () => {
    setSelectedResults(new Set());
    onClose();
  };

  const handleCollapse = () => {
    onCollapse();
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 20 }}
          className="fixed bottom-4 right-4 z-50 w-96 max-h-[80vh] bg-bg-card border border-border-light rounded-xl shadow-2xl overflow-hidden"
        >
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-secondary/50">
            <div className="flex items-center gap-2">
              <button
                onClick={handleCollapse}
                className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
                title="收起面板"
              >
                <ArrowLeft size={16} />
              </button>
              <span className="text-sm font-medium text-text-primary">
                搜索结果 ({searchResults.length})
              </span>
            </div>
            <button
              onClick={handleClose}
              className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
              title="关闭搜索"
            >
              <X size={16} />
            </button>
          </div>

          {/* Content */}
          <div className="flex-1 overflow-y-auto max-h-[60vh]">
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
                    onClick={handleSelectAll}
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
              <div className="space-y-1">
                {searchResults.map((result, i) => (
                  <motion.div
                    key={i}
                    initial={{ opacity: 0, y: 4 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: i * 0.03 }}
                    className={cn(
                      'flex items-start gap-2.5 p-2.5 rounded-lg cursor-pointer transition-all border',
                      selectedResults.has(i)
                        ? 'bg-accent/5 border-accent/20'
                        : 'hover:bg-bg-hover border-transparent'
                    )}
                    onClick={() => handleToggleResult(i)}
                  >
                    <div
                      className={cn(
                        'mt-0.5 w-4 h-4 rounded border-2 flex items-center justify-center flex-shrink-0 transition-all',
                        selectedResults.has(i)
                          ? 'bg-accent border-accent'
                          : 'border-border-light'
                      )}
                    >
                      {selectedResults.has(i) && <Check size={10} className="text-white" />}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-medium text-text-primary">{result.title}</p>
                      <p className="text-[10px] text-text-muted truncate mt-0.5">{result.url}</p>
                      {result.snippet && (
                        <p className="text-[10px] text-text-muted mt-1 line-clamp-2">
                          {result.snippet}
                        </p>
                      )}
                      {result.score > 0 && (
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-[10px] text-accent">
                            评分 {result.score.toFixed(1)}
                          </span>
                          {result.reason && (
                            <span className="text-[10px] text-text-muted">
                              · {result.reason}
                            </span>
                          )}
                        </div>
                      )}
                    </div>
                  </motion.div>
                ))}
              </div>

              {/* Empty state */}
              {searchResults.length === 0 && (
                <div className="flex flex-col items-center justify-center py-8 text-text-muted">
                  <p className="text-xs">{searchSummary || '未找到相关结果'}</p>
                </div>
              )}
            </div>
          </div>

          {/* Footer with import button */}
          {selectedResults.size > 0 && (
            <div className="border-t border-border p-3 bg-bg-secondary/50">
              <Button
                size="sm"
                className="w-full"
                onClick={handleImport}
              >
                <Plus size={13} /> 导入选中 ({selectedResults.size})
              </Button>
            </div>
          )}
        </motion.div>
      )}
    </AnimatePresence>
  );
}