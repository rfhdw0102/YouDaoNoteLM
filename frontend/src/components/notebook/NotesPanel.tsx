import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import {
  FileText, Map, HelpCircle, Presentation, Search,
  MoreHorizontal, Trash2, Edit3, Download, ArrowLeft,
  Copy, Eye, Code, BookOpen, Loader2, X, AlertCircle, Globe, ShieldOff
} from 'lucide-react';
import { useNotebookStore } from '../../stores/useNotebookStore';
import { cn } from '../../utils/cn';
import { formatDate } from '../../utils/format';
import type { Note, NoteType } from '../../types';
import { exportGenerationFile, downloadBlob } from '../../api/generation';
import MindmapViewer from './MindmapViewer';
import QuizCard from './QuizCard';
import PPTViewer from './PPTViewer';
import Button from '../ui/Button';

const typeIcons: Record<NoteType, typeof FileText> = {
  note: FileText,
  mindmap: Map,
  quiz: HelpCircle,
  ppt: Presentation,
};

const typeLabels: Record<NoteType, string> = {
  note: '笔记',
  mindmap: '思维导图',
  quiz: '测验',
  ppt: 'PPT',
};

const typeColors: Record<NoteType, string> = {
  note: 'bg-blue-500/10 text-blue-400',
  mindmap: 'bg-teal/10 text-teal',
  quiz: 'bg-orange-500/10 text-orange-400',
  ppt: 'bg-purple-500/10 text-purple-400',
};

export default function NotesPanel() {
  const { currentNotebookId, getCurrentNotebook, deleteNote, renameNote, toggleNoteSource, addNote, generateNote, generatingType, generationError, clearGenerationError } = useNotebookStore();
  const notebook = getCurrentNotebook();

  const [searchQuery, setSearchQuery] = useState('');
  const [selectedNote, setSelectedNote] = useState<Note | null>(null);
  const [viewMode, setViewMode] = useState<'visual' | 'source'>('visual');
  const [contextMenuId, setContextMenuId] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [genPrompt, setGenPrompt] = useState('');
  const [useWeb, setUseWeb] = useState(true);
  const [allowDegrade, setAllowDegrade] = useState(true);

  if (!notebook || !currentNotebookId) return null;

  const filteredNotes = notebook.notes.filter((n) =>
    n.title.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const handleStartRename = (id: string, title: string) => {
    setEditingId(id);
    setEditTitle(title);
    setContextMenuId(null);
  };

  const handleFinishRename = () => {
    if (editingId && editTitle.trim()) {
      renameNote(currentNotebookId, editingId, editTitle.trim());
    }
    setEditingId(null);
  };

  const handleCopy = (content: string) => {
    navigator.clipboard.writeText(content);
  };

  const handleDownload = async (note: Note) => {
    // PPT 类型调用后端导出 API 生成真正的 .pptx 文件
    if (note.type === 'ppt') {
      try {
        const file = await exportGenerationFile({
          type: note.type,
          content: note.content,
          title: note.title,
        });
        downloadBlob(file.blob, file.filename);
      } catch (err) {
        console.error('PPT export failed:', err);
      }
      return;
    }
    const blob = new Blob([note.content], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${note.title}.md`;
    a.click();
    URL.revokeObjectURL(url);
  };

  // 调用后端生成 Agent
  const handleGenerate = async (type: NoteType) => {
    if (generatingType || !currentNotebookId) return;
    await generateNote(currentNotebookId, type, {
      prompt: genPrompt.trim() || undefined,
      useWeb,
      allowDegrade,
    });
  };

  // ---- Note Viewer ----
  if (selectedNote) {
    return (
      <div className="h-full flex flex-col">
        <div className="flex items-center justify-between px-4 py-3 border-b border-border flex-shrink-0">
          <div className="flex items-center gap-2">
            <button onClick={() => setSelectedNote(null)} className="p-1 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer">
              <ArrowLeft size={16} />
            </button>
            <div className={cn('p-1.5 rounded-md', typeColors[selectedNote.type])}>
              {(() => { const Icon = typeIcons[selectedNote.type]; return <Icon size={13} />; })()}
            </div>
            <span className="text-sm font-medium text-text-primary truncate max-w-[200px]">{selectedNote.title}</span>
          </div>
          <div className="flex items-center gap-1">
            {(selectedNote.type === 'mindmap' || selectedNote.type === 'note') && (
              <div className="flex items-center bg-bg-tertiary rounded-md p-0.5 mr-1">
                <button onClick={() => setViewMode('visual')} className={cn('px-2 py-1 rounded text-xs transition-colors cursor-pointer', viewMode === 'visual' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary')}>
                  <Eye size={11} className="inline mr-1" />可视化
                </button>
                <button onClick={() => setViewMode('source')} className={cn('px-2 py-1 rounded text-xs transition-colors cursor-pointer', viewMode === 'source' ? 'bg-accent text-white' : 'text-text-muted hover:text-text-primary')}>
                  <Code size={11} className="inline mr-1" />源码
                </button>
              </div>
            )}
            <Button variant="ghost" size="sm" onClick={() => handleCopy(selectedNote.content)}><Copy size={12} /></Button>
            <Button variant="ghost" size="sm" onClick={() => handleDownload(selectedNote)}><Download size={12} /></Button>
          </div>
        </div>

        <div className="flex-1 overflow-auto">
          {selectedNote.type === 'mindmap' && viewMode === 'visual' && <MindmapViewer content={selectedNote.content} />}
          {selectedNote.type === 'quiz' && <QuizCard content={selectedNote.content} />}
          {selectedNote.type === 'ppt' && <PPTViewer content={selectedNote.content} />}
          {selectedNote.type === 'note' && viewMode === 'visual' && (
            <div className="p-6">
              <div className="bg-bg-card rounded-xl border border-border-light p-6 prose prose-sm max-w-none">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{selectedNote.content}</ReactMarkdown>
              </div>
            </div>
          )}
          {(() => {
            const showSource = selectedNote.type !== 'ppt' && selectedNote.type !== 'quiz' && viewMode === 'source';
            return showSource ? (
              <div className="p-6">
                <div className="bg-bg-card rounded-xl border border-border-light p-6">
                  <pre className="whitespace-pre-wrap text-sm text-text-secondary leading-relaxed font-[family-name:var(--font-mono)]">{selectedNote.content}</pre>
                </div>
              </div>
            ) : null;
          })()}
        </div>
      </div>
    );
  }

  // ---- Main: Generation buttons + Notes list ----
  return (
    <div className="h-full flex flex-col">
      {/* Generation buttons - top section */}
      <div className="px-4 pt-4 pb-3 border-b border-border flex-shrink-0">
        <p className="text-xs text-text-muted mb-2.5 font-medium">工作台</p>

        {/* 错误提示 */}
        <AnimatePresence>
          {generationError && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <div className="mb-2.5 flex items-start gap-2 rounded-lg border border-error/25 bg-error/8 px-3 py-2 text-xs text-error">
                <AlertCircle size={14} className="flex-shrink-0 mt-px" />
                <span className="flex-1 leading-relaxed">{generationError}</span>
                <button
                  onClick={clearGenerationError}
                  className="flex-shrink-0 p-0.5 rounded hover:bg-error/15 transition-colors cursor-pointer text-error/60 hover:text-error"
                >
                  <X size={12} />
                </button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        <div className="grid grid-cols-4 gap-2">
          {[
            { icon: Map, label: '思维导图', type: 'mindmap' as NoteType, color: 'from-teal to-emerald-400' },
            { icon: Presentation, label: 'PPT', type: 'ppt' as NoteType, color: 'from-purple-500 to-pink-400' },
            { icon: HelpCircle, label: '测验', type: 'quiz' as NoteType, color: 'from-orange-400 to-amber-400' },
            { icon: FileText, label: '笔记', type: 'note' as NoteType, color: 'from-blue-400 to-cyan-400' },
          ].map(({ icon: Icon, label, type, color }) => {
            const isActive = generatingType === type;
            const isDisabled = generatingType !== null && generatingType !== type;
            return (
              <button
                key={type}
                onClick={() => handleGenerate(type)}
                disabled={generatingType !== null}
                className={cn(
                  'flex flex-col items-center gap-1.5 p-3 rounded-xl border transition-all group',
                  isActive
                    ? 'border-accent/40 bg-accent/10'
                    : isDisabled
                      ? 'border-border-light bg-bg-secondary/50 opacity-50 cursor-not-allowed'
                      : 'border-border-light hover:border-accent/40 hover:bg-accent/5 cursor-pointer'
                )}
              >
                <div className={cn('w-9 h-9 rounded-lg bg-gradient-to-br flex items-center justify-center', color)}>
                  {isActive ? <Loader2 size={16} className="text-white animate-spin" /> : <Icon size={16} className="text-white" />}
                </div>
                <span className={cn(
                  'text-xs transition-colors',
                  isActive ? 'text-accent font-medium' : 'text-text-secondary group-hover:text-text-primary'
                )}>
                  {isActive ? '生成中...' : label}
                </span>
              </button>
            );
          })}
        </div>

        {/* 自定义提示词 + 选项开关 */}
        <div className="mt-2.5 space-y-2">
          <div className="relative">
            <input
              type="text"
              value={genPrompt}
              onChange={(e) => setGenPrompt(e.target.value)}
              placeholder={'输入自定义提示词，如「聚焦第三章核心概念」...'}              disabled={generatingType !== null}
              className={cn(
                'w-full h-8 px-3 pr-8 rounded-lg border text-xs outline-none transition-colors',
                'bg-bg-secondary border-border text-text-primary placeholder:text-text-muted',
                'focus:border-accent/50 focus:bg-bg-card',
                generatingType !== null && 'opacity-50 cursor-not-allowed'
              )}
            />
            {genPrompt && !generatingType && (
              <button
                onClick={() => setGenPrompt('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 rounded text-text-muted hover:text-text-primary cursor-pointer"
              >
                <X size={12} />
              </button>
            )}
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={() => setUseWeb(!useWeb)}
              disabled={generatingType !== null}
              className={cn(
                'flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] border transition-all',
                useWeb
                  ? 'bg-accent/10 border-accent/30 text-accent'
                  : 'bg-bg-tertiary border-border text-text-muted',
                generatingType !== null && 'opacity-50 cursor-not-allowed',
                generatingType === null && 'cursor-pointer hover:border-accent/30'
              )}
            >
              <Globe size={11} />
              联网搜索
            </button>

            <button
              onClick={() => setAllowDegrade(!allowDegrade)}
              disabled={generatingType !== null}
              className={cn(
                'flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] border transition-all',
                allowDegrade
                  ? 'bg-accent/10 border-accent/30 text-accent'
                  : 'bg-bg-tertiary border-border text-text-muted',
                generatingType !== null && 'opacity-50 cursor-not-allowed',
                generatingType === null && 'cursor-pointer hover:border-accent/30'
              )}
            >
              <ShieldOff size={11} />
              允许降级
            </button>
          </div>
        </div>
      </div>

      {/* Search */}
      <div className="px-3 py-2 border-b border-border flex-shrink-0">
        <div className="relative">
          <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted" />
          <input
            type="text"
            placeholder="搜索笔记..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full h-7 pl-8 pr-3 rounded-md bg-bg-tertiary border border-border text-xs text-text-primary placeholder:text-text-muted focus:border-accent focus:outline-none"
          />
        </div>
      </div>

      {/* Notes list */}
      <div className="flex-1 overflow-y-auto py-1">
        <AnimatePresence mode="popLayout">
          {filteredNotes.map((note) => {
            const Icon = typeIcons[note.type];
            return (
              <motion.div
                key={note.id}
                layout
                initial={{ opacity: 0, x: 10 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 10 }}
                className="group relative mx-1.5 mb-1"
              >
                <div
                  className="flex items-start gap-2.5 p-3 rounded-lg cursor-pointer transition-all hover:bg-bg-hover border border-transparent hover:border-border-light"
                  onClick={() => setSelectedNote(note)}
                >
                  <div className={cn('p-1.5 rounded-md flex-shrink-0 mt-0.5', typeColors[note.type])}>
                    <Icon size={14} />
                  </div>
                  <div className="flex-1 min-w-0">
                    {editingId === note.id ? (
                      <input
                        autoFocus
                        value={editTitle}
                        onChange={(e) => setEditTitle(e.target.value)}
                        onBlur={handleFinishRename}
                        onKeyDown={(e) => { if (e.key === 'Enter') handleFinishRename(); }}
                        className="w-full bg-transparent text-xs outline-none border-b border-accent"
                        onClick={(e) => e.stopPropagation()}
                      />
                    ) : (
                      <p className="text-xs font-medium text-text-primary truncate">{note.title}</p>
                    )}
                    <div className="flex items-center gap-2 mt-1">
                      <span className="text-[10px] text-accent bg-accent-glow px-1.5 py-0.5 rounded-full">{typeLabels[note.type]}</span>
                      <span className="text-[10px] text-text-muted">{formatDate(note.updatedAt)}</span>
                      {note.isSource && <span className="text-[10px] text-teal bg-teal-glow px-1.5 py-0.5 rounded-full">已设为来源</span>}
                    </div>
                  </div>
                  <button
                    onClick={(e) => { e.stopPropagation(); setContextMenuId(contextMenuId === note.id ? null : note.id); }}
                    className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-bg-active transition-all cursor-pointer"
                  >
                    <MoreHorizontal size={13} />
                  </button>

                  {contextMenuId === note.id && (
                    <>
                      <div className="fixed inset-0 z-40" onClick={() => setContextMenuId(null)} />
                      <div className="absolute right-0 top-full mt-1 w-40 bg-bg-card border border-border-light rounded-lg shadow-xl z-50 py-1">
                        <button onClick={(e) => { e.stopPropagation(); handleStartRename(note.id, note.title); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-text-secondary hover:bg-bg-hover cursor-pointer">
                          <Edit3 size={11} /> 重命名
                        </button>
                        <button onClick={(e) => { e.stopPropagation(); toggleNoteSource(currentNotebookId, note.id); setContextMenuId(null); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-text-secondary hover:bg-bg-hover cursor-pointer">
                          <BookOpen size={11} /> {note.isSource ? '取消来源' : '设为来源'}
                        </button>
                        <button onClick={(e) => { e.stopPropagation(); handleDownload(note); setContextMenuId(null); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-text-secondary hover:bg-bg-hover cursor-pointer">
                          <Download size={11} /> {note.type === 'ppt' ? '下载 .pptx' : '下载 .md'}
                        </button>
                        <div className="border-t border-border mt-1 pt-1">
                          <button onClick={(e) => { e.stopPropagation(); deleteNote(currentNotebookId, note.id); setContextMenuId(null); }} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-error hover:bg-error/5 cursor-pointer">
                            <Trash2 size={11} /> 删除
                          </button>
                        </div>
                      </div>
                    </>
                  )}
                </div>
              </motion.div>
            );
          })}
        </AnimatePresence>

        {filteredNotes.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-text-muted">
            <FileText size={32} className="mb-3 opacity-30" />
            <p className="text-xs">暂无笔记</p>
            <p className="text-xs mt-1">点击上方按钮生成</p>
          </div>
        )}
      </div>
    </div>
  );
}
