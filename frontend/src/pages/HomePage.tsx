import { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Plus, BookOpen, Clock,
  Search, MoreHorizontal, Trash2, Edit3, Loader2
} from 'lucide-react';
import { useNotebookStore } from '../stores/useNotebookStore';
import { formatDate } from '../utils/format';
import Button from '../components/ui/Button';
import Modal from '../components/ui/Modal';
import Input from '../components/ui/Input';

export default function HomePage() {
  const navigate = useNavigate();
  const { notebooks, loading, fetchNotebooks, createNotebook, deleteNotebook, renameNotebook } = useNotebookStore();

  useEffect(() => {
    fetchNotebooks();
  }, [fetchNotebooks]);
  const [showNewModal, setShowNewModal] = useState(false);
  const [newName, setNewName] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [menuNotebookId, setMenuNotebookId] = useState<string | null>(null);
  const [menuPos, setMenuPos] = useState({ x: 0, y: 0 });
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [error, setError] = useState('');
  const menuRef = useRef<HTMLDivElement>(null);

  const filtered = notebooks.filter((n) =>
    n.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  useEffect(() => {
    if (!menuNotebookId) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuNotebookId(null);
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [menuNotebookId]);

  const handleCreate = async () => {
    if (!newName.trim()) return;
    setError('');
    try {
      await createNotebook(newName.trim());
      const { currentNotebookId } = useNotebookStore.getState();
      setNewName('');
      setShowNewModal(false);
      if (currentNotebookId) {
        navigate(`/notebook/${currentNotebookId}`);
      }
    } catch (err: any) {
      setError(err.message || '创建失败');
    }
  };

  const handleOpenMenu = (e: React.MouseEvent, notebookId: string) => {
    e.stopPropagation();
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setMenuPos({ x: rect.right, y: rect.bottom + 4 });
    setMenuNotebookId(menuNotebookId === notebookId ? null : notebookId);
  };

  const handleStartRename = (id: string, name: string) => {
    setRenamingId(id); setRenameValue(name); setMenuNotebookId(null); setError('');
  };

  const handleRename = async () => {
    if (!renamingId || !renameValue.trim()) return;
    setError('');
    try {
      await renameNotebook(renamingId, renameValue.trim());
      setRenamingId(null);
    } catch (err: any) {
      setError(err.message || '重命名失败');
    }
  };

  return (
    <div className="h-full overflow-y-auto bg-bg-primary">
      <div className="max-w-[1400px] mx-auto px-8 py-8">
        {/* Header row */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <h1 className="text-lg font-semibold text-text-primary">我的笔记本</h1>
            <span className="text-xs text-text-muted bg-bg-hover px-2 py-0.5 rounded-full">{notebooks.length}</span>
          </div>
          <div className="flex items-center gap-3">
            <div className="relative">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-text-muted" />
              <input type="text" placeholder="搜索笔记本..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)}
                className="h-9 pl-9 pr-4 rounded-lg bg-bg-card border border-border text-sm text-text-primary placeholder:text-text-muted focus:border-accent focus:outline-none w-56" />
            </div>
            <Button onClick={() => setShowNewModal(true)}><Plus size={16} /> 新建笔记本</Button>
          </div>
        </div>

        {/* Loading state */}
        {loading && notebooks.length === 0 && (
          <div className="flex items-center justify-center py-20">
            <Loader2 size={24} className="animate-spin text-accent" />
            <span className="ml-2 text-sm text-text-muted">加载中...</span>
          </div>
        )}

        {/* Grid - 4 per row */}
        {!loading && <div className="grid grid-cols-4 gap-4">
          {/* New notebook card */}
          <motion.button initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }}
            onClick={() => setShowNewModal(true)}
            className="flex flex-col items-center justify-center gap-2 p-6 rounded-xl border-2 border-dashed border-border-light hover:border-accent/40 hover:bg-accent/5 transition-all cursor-pointer group min-h-[160px]">
            <div className="w-10 h-10 rounded-lg bg-bg-hover group-hover:bg-accent/10 flex items-center justify-center transition-colors">
              <Plus size={20} className="text-text-muted group-hover:text-accent transition-colors" />
            </div>
            <span className="text-xs text-text-muted group-hover:text-accent transition-colors">创建新笔记本</span>
          </motion.button>

          {filtered.map((notebook, i) => (
            <motion.div key={notebook.id} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: i * 0.03 }}
              className="group relative bg-bg-card rounded-xl border border-border-light hover:border-accent/30 hover:shadow-lg hover:shadow-accent/5 transition-all cursor-pointer overflow-visible min-h-[140px]"
              onClick={() => navigate(`/notebook/${notebook.id}`)}>
              <div className="h-1 bg-gradient-to-r from-accent to-teal rounded-t-xl" />
              <div className="p-4 flex flex-col h-full">
                <div className="flex items-start justify-between mb-2">
                  <div className="w-9 h-9 rounded-lg bg-accent/10 flex items-center justify-center">
                    <BookOpen size={16} className="text-accent" />
                  </div>
                  <button onClick={(e) => handleOpenMenu(e, notebook.id)}
                    className="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-bg-hover transition-all cursor-pointer">
                    <MoreHorizontal size={14} className="text-text-muted" />
                  </button>
                </div>
                <h3 className="text-sm font-semibold text-text-primary mb-1 group-hover:text-accent transition-colors truncate">{notebook.name}</h3>
                {notebook.description && <p className="text-xs text-text-muted mb-3 line-clamp-1">{notebook.description}</p>}
                <div className="mt-auto flex justify-end">
                  <span className="text-[11px] text-text-muted flex items-center gap-1"><Clock size={10} /> {formatDate(notebook.updatedAt)}</span>
                </div>
              </div>
            </motion.div>
          ))}
        </div>}
      </div>

      {/* Context Menu */}
      <AnimatePresence>
        {menuNotebookId && (
          <motion.div ref={menuRef} initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.95 }}
            className="fixed w-40 bg-bg-card border border-border-light rounded-xl shadow-2xl z-[100] py-1.5 overflow-hidden"
            style={{ left: menuPos.x - 160, top: menuPos.y }}>
            <button onMouseDown={(e) => { e.preventDefault(); e.stopPropagation(); const nb = notebooks.find(n => n.id === menuNotebookId); if (nb) handleStartRename(nb.id, nb.name); }}
              className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-text-secondary hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer">
              <Edit3 size={14} /> 重命名
            </button>
            <div className="border-t border-border my-0.5" />
            <button onMouseDown={(e) => { e.preventDefault(); e.stopPropagation(); if (menuNotebookId) { deleteNotebook(menuNotebookId); setMenuNotebookId(null); } }}
              className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-error hover:bg-error/5 transition-colors cursor-pointer">
              <Trash2 size={14} /> 删除
            </button>
          </motion.div>
        )}
      </AnimatePresence>

      <Modal open={showNewModal} onClose={() => { setShowNewModal(false); setError(''); }} title="新建笔记本" size="sm">
        <div className="space-y-4">
          <Input label="笔记本名称" placeholder="输入笔记本名称..." value={newName} onChange={(e) => { setNewName(e.target.value); setError(''); }} onKeyDown={(e) => e.key === 'Enter' && handleCreate()} />
          {error && <p className="text-sm text-error">{error}</p>}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => { setShowNewModal(false); setError(''); }}>取消</Button>
            <Button onClick={handleCreate} disabled={!newName.trim()}>创建</Button>
          </div>
        </div>
      </Modal>

      {/* Rename Modal */}
      <Modal open={!!renamingId} onClose={() => { setRenamingId(null); setError(''); }} title="重命名笔记本" size="sm">
        <div className="space-y-4">
          <Input label="笔记本名称" placeholder="输入新名称..." value={renameValue}
            onChange={(e) => { setRenameValue(e.target.value); setError(''); }}
            onKeyDown={(e) => e.key === 'Enter' && handleRename()} />
          {error && <p className="text-sm text-error">{error}</p>}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => { setRenamingId(null); setError(''); }}>取消</Button>
            <Button onClick={handleRename} disabled={!renameValue.trim()}>确定</Button>
          </div>
        </div>
      </Modal>

      {/* Error toast */}
      <AnimatePresence>
        {error && !showNewModal && !renamingId && (
          <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 20 }}
            className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2.5 rounded-xl bg-error/10 border border-error/20 text-sm text-error shadow-lg">
            {error}
            <button onClick={() => setError('')} className="ml-3 text-error/60 hover:text-error cursor-pointer">✕</button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
