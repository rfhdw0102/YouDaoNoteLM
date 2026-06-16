import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  Folder, ChevronRight, ArrowLeft,
  Loader2, Check, Square, SquareCheck, AlertCircle, RefreshCw
} from 'lucide-react';
import { cn } from '../../utils/cn';
import Button from '../ui/Button';
import Badge from '../ui/Badge';
import * as youdaoApi from '../../api/youdao';
import type { YoudaoNoteItem } from '../../api/youdao';

interface YoudaoImportPanelProps {
  onImport: (fileIds: string[], fileNames: Record<string, string>) => Promise<void>;
  onBack: () => void;
}

export default function YoudaoImportPanel({ onImport, onBack }: YoudaoImportPanelProps) {
  const [notes, setNotes] = useState<YoudaoNoteItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedNotes, setSelectedNotes] = useState<Set<string>>(new Set());
  const [currentFolder, setCurrentFolder] = useState<string | null>(null);
  const [folderPath, setFolderPath] = useState<{ id: string; name: string }[]>([]);
  const [importing, setImporting] = useState(false);

  // 加载有道云笔记数据
  useEffect(() => {
    loadNotes(currentFolder);
  }, [currentFolder]);

  const loadNotes = async (folderId: string | null) => {
    setLoading(true);
    setError(null);

    try {
      const response = await youdaoApi.listNotes(folderId || undefined);

      if (response.code === 0) {
        // 后端已修复解析逻辑，直接使用返回的数据
        setNotes(response.data || []);
      } else {
        setError(response.message || '加载失败');
      }
    } catch (err) {
      setError('加载有道云笔记失败');
      console.error('Failed to load youdao notes:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleFolderClick = (folderId: string, folderName: string) => {
    setLoading(true);  // 立即显示加载状态，避免闪烁
    setNotes([]);  // 清空当前笔记列表，避免显示旧数据
    setSelectedNotes(new Set());  // 清空选中状态
    setCurrentFolder(folderId);
    setFolderPath([...folderPath, { id: folderId, name: folderName }]);
  };

  const handleNoteSelect = (noteId: string) => {
    const newSelected = new Set(selectedNotes);
    if (newSelected.has(noteId)) {
      newSelected.delete(noteId);
    } else {
      newSelected.add(noteId);
    }
    setSelectedNotes(newSelected);
  };

  const handleSelectAll = () => {
    const fileNotes = notes.filter(n => n.type === 'file');
    if (selectedNotes.size === fileNotes.length) {
      setSelectedNotes(new Set());
    } else {
      setSelectedNotes(new Set(fileNotes.map(n => n.id)));
    }
  };

  const handleImport = async () => {
    if (selectedNotes.size === 0) return;

    setImporting(true);
    try {
      // 构建 fileID -> name 映射
      const fileNames: Record<string, string> = {};
      for (const note of notes) {
        if (selectedNotes.has(note.id)) {
          fileNames[note.id] = note.name;
        }
      }
      await onImport(Array.from(selectedNotes), fileNames);
    } catch (err) {
      setError('导入失败');
      console.error('Import failed:', err);
    } finally {
      setImporting(false);
    }
  };

  // 所有文件类型都支持导入（后端会自动处理 .note 格式转换）
  const fileNotes = notes ? notes.filter(n => n.type === 'file') : [];
  const allSelected = fileNotes.length > 0 && selectedNotes.size === fileNotes.length;

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-3 border-b border-border flex-shrink-0">
        <button
          onClick={onBack}
          className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
        >
          <ArrowLeft size={16} />
        </button>
        <div className="flex-1">
          <h3 className="text-sm font-medium text-text-primary">导入有道云笔记</h3>
          <p className="text-xs text-text-muted">
            {folderPath.length > 0 ? folderPath[folderPath.length - 1].name : '根目录'}
          </p>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => loadNotes(currentFolder)}
          disabled={loading}
        >
          <RefreshCw size={14} className={cn(loading && 'animate-spin')} />
        </Button>
      </div>

      {/* Breadcrumb */}
      {folderPath.length > 0 && (
        <div className="px-4 py-2 border-b border-border flex-shrink-0">
          <div className="flex items-center gap-1 text-xs text-text-muted">
            <button
              onClick={() => {
                setLoading(true);  // 立即显示加载状态
                setNotes([]);  // 清空当前笔记列表
                setSelectedNotes(new Set());
                setFolderPath([]);
                setCurrentFolder(null);
              }}
              className="hover:text-accent cursor-pointer"
            >
              根目录
            </button>
            {folderPath.map((folder, index) => (
              <span key={folder.id} className="flex items-center gap-1">
                <ChevronRight size={12} />
                <button
                  onClick={() => {
                    setLoading(true);  // 立即显示加载状态
                    setNotes([]);  // 清空当前笔记列表
                    setSelectedNotes(new Set());
                    const newPath = folderPath.slice(0, index + 1);
                    setFolderPath(newPath);
                    setCurrentFolder(folder.id);
                  }}
                  className="hover:text-accent cursor-pointer"
                >
                  {folder.name}
                </button>
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 size={20} className="animate-spin text-accent" />
            <span className="ml-2 text-sm text-text-muted">加载中...</span>
          </div>
        ) : error ? (
          <div className="flex flex-col items-center justify-center py-12 gap-3">
            <AlertCircle size={24} className="text-error" />
            <p className="text-sm text-error">{error}</p>
            <Button size="sm" onClick={() => loadNotes(currentFolder)}>
              重试
            </Button>
          </div>
        ) : !notes || notes.length === 0 ? (
          <div className="text-center py-12 text-text-muted">
            <p className="text-sm">暂无笔记</p>
          </div>
        ) : (
          <div className="space-y-2">
            {/* Select all */}
            {fileNotes.length > 0 && (
              <div className="flex items-center gap-2 px-3 py-2 bg-bg-tertiary rounded-lg">
                <button
                  onClick={handleSelectAll}
                  className="cursor-pointer"
                >
                  {allSelected ? (
                    <SquareCheck size={16} className="text-accent" />
                  ) : (
                    <Square size={16} className="text-text-muted" />
                  )}
                </button>
                <span className="text-xs text-text-muted">
                  {allSelected ? '取消全选' : '全选所有笔记'}
                </span>
                <span className="text-xs text-text-muted ml-auto">
                  {selectedNotes.size} / {fileNotes.length} 已选
                </span>
              </div>
            )}

            {/* Notes list */}
            {notes && notes.map((note) => (
              <motion.div
                key={note.id}
                initial={{ opacity: 0, y: 5 }}
                animate={{ opacity: 1, y: 0 }}
                className={cn(
                  'flex items-center gap-3 px-3 py-2 rounded-lg border transition-all cursor-pointer',
                  selectedNotes.has(note.id)
                    ? 'border-accent/30 bg-accent/5'
                    : 'border-border-light hover:border-accent/20 hover:bg-bg-hover'
                )}
                onClick={() => {
                  if (note.type === 'dir') {
                    handleFolderClick(note.id, note.name);
                  } else {
                    handleNoteSelect(note.id);
                  }
                }}
              >
                {note.type === 'dir' ? (
                  <Folder size={16} className="text-accent flex-shrink-0" />
                ) : (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleNoteSelect(note.id);
                    }}
                    className="cursor-pointer flex-shrink-0"
                  >
                    {selectedNotes.has(note.id) ? (
                      <SquareCheck size={16} className="text-accent" />
                    ) : (
                      <Square size={16} className="text-text-muted" />
                    )}
                  </button>
                )}

                <div className="flex-1 min-w-0">
                  <p className="text-sm text-text-primary truncate">
                    {note.name}
                  </p>
                  <p className="text-xs text-text-muted">
                    {note.type === 'dir' ? '文件夹' : '笔记'}
                  </p>
                </div>

                {note.type === 'dir' && (
                  <ChevronRight size={14} className="text-text-muted" />
                )}
              </motion.div>
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="flex-shrink-0 p-4 border-t border-border">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Badge variant="accent">
              {selectedNotes.size} 个笔记已选
            </Badge>
          </div>
          <Button
            size="sm"
            onClick={handleImport}
            disabled={selectedNotes.size === 0 || importing}
          >
            {importing ? (
              <>
                <Loader2 size={14} className="animate-spin mr-1" />
                导入中...
              </>
            ) : (
              <>
                <Check size={14} className="mr-1" />
                导入选中笔记
              </>
            )}
          </Button>
        </div>
        <p className="text-xs text-text-muted">
          导入后将自动进行向量化处理，支持 AI 对话和搜索
        </p>
      </div>
    </div>
  );
}