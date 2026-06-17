import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  FileText, Map, HelpCircle, Presentation, Search,
  MoreHorizontal, Trash2, Edit3, Download, ArrowLeft,
  Copy, Eye, Code, BookOpen
} from 'lucide-react';
import { useNotebookStore } from '../../stores/useNotebookStore';
import { cn } from '../../utils/cn';
import { formatDate } from '../../utils/format';
import type { Note, NoteType } from '../../types';
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
  const { currentNotebookId, getCurrentNotebook, deleteNote, renameNote, toggleNoteSource, addNote } = useNotebookStore();
  const notebook = getCurrentNotebook();

  const [searchQuery, setSearchQuery] = useState('');
  const [selectedNote, setSelectedNote] = useState<Note | null>(null);
  const [viewMode, setViewMode] = useState<'visual' | 'source'>('visual');
  const [contextMenuId, setContextMenuId] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');

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

  const handleDownload = (note: Note) => {
    const blob = new Blob([note.content], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${note.title}.md`;
    a.click();
    URL.revokeObjectURL(url);
  };

  // Quick generate a mock note
  const handleGenerate = (type: NoteType) => {
    const typeNames: Record<NoteType, string> = {
      mindmap: '思维导图',
      ppt: '演示文稿',
      quiz: '测验',
      note: '笔记',
    };
    const contents: Record<NoteType, string> = {
      mindmap: `# 知识框架\n\n## 概念一\n- 子概念 A\n- 子概念 B\n\n## 概念二\n- 子概念 C\n- 子概念 D\n\n## 概念三\n- 子概念 E\n- 子概念 F`,
      ppt: `<html><head><style>body{font-family:sans-serif;background:#0f1117;color:#E8E6E3;margin:0}.slide{width:100%;height:100vh;display:flex;flex-direction:column;justify-content:center;align-items:center;padding:60px;box-sizing:border-box}h1{font-size:48px;background:linear-gradient(135deg,#6C63FF,#4ECDC4);-webkit-background-clip:text;-webkit-text-fill-color:transparent}h2{font-size:36px;color:#6C63FF}p,li{font-size:20px;line-height:1.8}.slide{border-bottom:1px solid #1E2130}</style></head><body><div class="slide"><h1>演示文稿</h1><p>基于资料生成</p></div><div class="slide"><h2>要点一</h2><p>核心内容描述</p></div><div class="slide"><h2>要点二</h2><p>详细分析</p></div></body></html>`,
      quiz: JSON.stringify({
        questions: [
          { id: 'q1', question: '示例问题 1？', options: ['选项 A', '选项 B', '选项 C', '选项 D'], correctIndex: 0, explanation: '这是解析。' },
          { id: 'q2', question: '示例问题 2？', options: ['选项 A', '选项 B', '选项 C', '选项 D'], correctIndex: 2, explanation: '这是解析。' },
        ],
      }),
      note: `# 结构化笔记\n\n## 核心要点\n\n1. 要点一：详细描述\n2. 要点二：详细描述\n3. 要点三：详细描述\n\n## 总结\n\n这是一份基于资料生成的结构化笔记。`,
    };
    const newNote: Note = {
      id: `note-${Date.now()}`,
      title: `新${typeNames[type]}`,
      type,
      content: contents[type],
      isSource: false,
      notebookId: currentNotebookId,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    addNote(currentNotebookId, newNote);
    setSelectedNote(newNote);
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
            {(selectedNote.type === 'mindmap' || selectedNote.type === 'ppt') && (
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

        <div className="flex-1 overflow-y-auto">
          {selectedNote.type === 'mindmap' && viewMode === 'visual' && <MindmapViewer content={selectedNote.content} />}
          {selectedNote.type === 'quiz' && <QuizCard content={selectedNote.content} />}
          {selectedNote.type === 'ppt' && viewMode === 'visual' && <PPTViewer content={selectedNote.content} />}
          {(() => {
            const showSource = selectedNote.type === 'note' || viewMode === 'source';
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
        <div className="grid grid-cols-4 gap-2">
          {[
            { icon: Map, label: '思维导图', type: 'mindmap' as NoteType, color: 'from-teal to-emerald-400' },
            { icon: Presentation, label: 'PPT', type: 'ppt' as NoteType, color: 'from-purple-500 to-pink-400' },
            { icon: HelpCircle, label: '测验', type: 'quiz' as NoteType, color: 'from-orange-400 to-amber-400' },
            { icon: FileText, label: '笔记', type: 'note' as NoteType, color: 'from-blue-400 to-cyan-400' },
          ].map(({ icon: Icon, label, type, color }) => (
            <button
              key={type}
              onClick={() => handleGenerate(type)}
              className={cn(
                'flex flex-col items-center gap-1.5 p-3 rounded-xl border border-border-light',
                'hover:border-accent/40 hover:bg-accent/5 transition-all cursor-pointer group'
              )}
            >
              <div className={cn('w-9 h-9 rounded-lg bg-gradient-to-br flex items-center justify-center', color)}>
                <Icon size={16} className="text-white" />
              </div>
              <span className="text-xs text-text-secondary group-hover:text-text-primary transition-colors">{label}</span>
            </button>
          ))}
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
                          <Download size={11} /> 下载 .md
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
