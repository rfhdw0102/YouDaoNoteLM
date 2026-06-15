import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { ArrowLeft, Edit3, Loader2 } from 'lucide-react';
import { useNotebookStore } from '../stores/useNotebookStore';
import SourcesPanel from '../components/notebook/SourcesPanel';
import ChatPanel from '../components/notebook/ChatPanel';
import NotesPanel from '../components/notebook/NotesPanel';
import ResizablePanel from '../components/ui/ResizablePanel';

export default function NotebookPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { setCurrentNotebook, getCurrentNotebook, renameNotebook, fetchNotebooks } = useNotebookStore();

  const [editingName, setEditingName] = useState(false);
  const [notebookName, setNotebookName] = useState('');
  const [renameError, setRenameError] = useState('');
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const loadNotebook = async () => {
      if (!id) return;

      setIsLoading(true);
      // 先确保笔记本列表已加载
      const { notebooks } = useNotebookStore.getState();
      if (notebooks.length === 0) {
        await fetchNotebooks();
      }
      await setCurrentNotebook(id);
      setIsLoading(false);
    };
    loadNotebook();
  }, [id, setCurrentNotebook, fetchNotebooks]);

  const notebook = getCurrentNotebook();

  useEffect(() => {
    if (notebook) setNotebookName(notebook.name);
  }, [notebook?.name]);

  if (isLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Loader2 size={24} className="animate-spin text-accent" />
        <span className="ml-2 text-sm text-text-muted">加载中...</span>
      </div>
    );
  }

  if (!notebook || !id) {
    return (
      <div className="h-full flex items-center justify-center">
        <p className="text-text-muted">笔记本不存在</p>
      </div>
    );
  }

  const handleFinishRename = async () => {
    if (notebookName.trim() && notebookName.trim() !== notebook.name) {
      try {
        await renameNotebook(id, notebookName.trim());
        setRenameError('');
      } catch (err: any) {
        setRenameError(err.message || '重命名失败');
        setNotebookName(notebook.name);
        return;
      }
    }
    setEditingName(false);
  };

  return (
    <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="h-full flex flex-col">
      {/* Notebook name bar */}
      <div className="flex items-center gap-3 px-4 py-2 border-b border-border bg-bg-secondary/30 flex-shrink-0">
        <button
          onClick={() => navigate('/')}
          className="p-1.5 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
        >
          <ArrowLeft size={16} />
        </button>
        {editingName ? (
          <input
            autoFocus
            value={notebookName}
            onChange={(e) => setNotebookName(e.target.value)}
            onBlur={handleFinishRename}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleFinishRename();
              if (e.key === 'Escape') { setNotebookName(notebook.name); setEditingName(false); }
            }}
            className="text-sm font-semibold text-text-primary bg-transparent outline-none border-b border-accent px-1 py-0.5"
          />
        ) : (
          <button
            onClick={() => setEditingName(true)}
            className="group flex items-center gap-1.5 text-sm font-semibold text-text-primary hover:text-accent transition-colors cursor-pointer"
          >
            {notebook.name}
            <Edit3 size={12} className="opacity-0 group-hover:opacity-100 text-text-muted transition-opacity" />
          </button>
        )}
      </div>

      {/* Three columns: left 25% | center 50% | right 25% */}
      <div className="flex-1 flex overflow-hidden">
        <ResizablePanel defaultWidth={25} minWidth={15} maxWidth={40} direction="right" className="border-r border-border overflow-hidden">
          <SourcesPanel />
        </ResizablePanel>
        <div className="flex-1 min-w-0 overflow-hidden">
          <ChatPanel />
        </div>
        <ResizablePanel defaultWidth={25} minWidth={15} maxWidth={40} direction="left" className="border-l border-border overflow-hidden">
          <NotesPanel />
        </ResizablePanel>
      </div>

      {/* Rename error toast */}
      <AnimatePresence>
        {renameError && (
          <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 20 }}
            className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2.5 rounded-xl bg-error/10 border border-error/20 text-sm text-error shadow-lg">
            {renameError}
            <button onClick={() => setRenameError('')} className="ml-3 text-error/60 hover:text-error cursor-pointer">✕</button>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
