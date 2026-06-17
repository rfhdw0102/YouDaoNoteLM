import { create } from 'zustand';
import type { Notebook, Source, Conversation, Note, ChatMessage, Reference } from '../types';
import * as notebookApi from '../api/notebook';
import * as sourceApi from '../api/source';
import * as importApi from '../api/import';
import * as searchApi from '../api/search';
import * as chatApi from '../api/chat';
import { getErrorMessage, getChatErrorMessage } from '../utils/error';

// Store the abort controller for the current streaming request
let currentStreamAbortController: AbortController | null = null;

interface NotebookState {
  notebooks: Notebook[];
  currentNotebookId: string | null;
  currentConversationId: string | null;
  loading: boolean;
  streamingContent: string;  // For real-time display
  // sourceID → taskID 映射，用于取消正在运行的导入任务
  taskIdBySourceId: Record<string, string>;

  // Init - fetch from API
  fetchNotebooks: () => Promise<void>;

  // Getters
  getCurrentNotebook: () => Notebook | undefined;
  getCurrentConversation: () => Conversation | undefined;
  getSelectedSources: () => Source[];

  // Notebook actions (API)
  setCurrentNotebook: (id: string) => Promise<void>;
  createNotebook: (name: string) => Promise<void>;
  deleteNotebook: (id: string) => Promise<void>;
  renameNotebook: (id: string, name: string) => Promise<void>;

  // Source actions (API-backed)
  fetchSources: (notebookId: string, page?: number, size?: number, keyword?: string) => Promise<void>;
  addSource: (notebookId: string, source: Source) => void;
  updateSource: (notebookId: string, sourceId: string, updates: Partial<Source>) => void;
  removeSource: (notebookId: string, sourceId: string) => Promise<void>;
  batchRemoveSources: (notebookId: string, sourceIds: string[]) => Promise<void>;
  deleteFailedSources: (notebookId: string) => Promise<number>;
  toggleSourceSelection: (notebookId: string, sourceId: string) => void;
  renameSource: (notebookId: string, sourceId: string, name: string) => Promise<void>;
  selectAllSources: (notebookId: string) => void;
  deselectAllSources: (notebookId: string) => void;
  fetchSourceContent: (notebookId: string, sourceId: string) => Promise<string>;
  fetchSourceOriginal: (notebookId: string, sourceId: string) => Promise<{ content: string; type: string }>;
  getSourceDownloadURL: (notebookId: string, sourceId: string) => Promise<string>;

  // Import actions (API)
  importFile: (notebookId: string, file: File) => Promise<Source>;
  previewAudio: (notebookId: string, file: File) => Promise<importApi.AudioPreviewData>;
  pollAudioPreview: (previewId: string) => Promise<importApi.AudioPreviewStatusData>;
  confirmAudio: (previewId: string, notebookId: string, content?: string) => Promise<Source>;
  getImportTask: (taskId: string) => Promise<importApi.ImportTaskData>;
  deleteImportTask: (taskId: string) => Promise<void>;

  // Search actions (API)
  searchSources: (notebookId: string, query: string) => Promise<searchApi.SearchResponseData>;
  searchSourcesStream: (
    notebookId: string,
    query: string,
    onEvent: (event: searchApi.SearchStreamEvent) => void,
    signal?: AbortSignal,
  ) => Promise<void>;
  importFromURL: (notebookId: string, url: string) => Promise<{ taskId: string; sourceId: number }>;
  importSearchResults: (notebookId: string, items: searchApi.SearchImportItem[]) => Promise<{ taskId: string; sourceIds: number[] }>;

  // Conversation actions (API-backed)
  fetchConversations: (notebookId: string) => Promise<void>;
  createConversation: (notebookId: string, title?: string) => Promise<string>;
  setCurrentConversation: (id: string) => void;
  deleteConversation: (notebookId: string, conversationId: string) => Promise<void>;
  renameConversation: (notebookId: string, conversationId: string, title: string) => Promise<void>;

  // Message actions (API-backed)
  fetchMessages: (notebookId: string, conversationId: string) => Promise<void>;
  sendMessage: (notebookId: string, conversationId: string, content: string, sourceIds?: number[], llmConfigId?: number) => Promise<void>;
  stopGeneration: (notebookId: string, conversationId: string) => Promise<void>;
  addMessage: (notebookId: string, conversationId: string, message: ChatMessage) => void;
  updateMessage: (notebookId: string, conversationId: string, messageId: string, updates: Partial<ChatMessage>) => void;

  // Note actions (local)
  addNote: (notebookId: string, note: Note) => void;
  deleteNote: (notebookId: string, noteId: string) => void;
  renameNote: (notebookId: string, noteId: string, title: string) => void;
  updateNoteContent: (notebookId: string, noteId: string, content: string) => void;
  toggleNoteSource: (notebookId: string, noteId: string) => void;

  // Reimport actions
  reimportAll: () => Promise<number>;
  reimportSelected: (sourceIds: string[]) => Promise<number>;
}

// Helper: convert backend SourceData to frontend Source
function toSource(s: sourceApi.SourceData): Source {
  // 后端状态映射：pending/processing → loading, ready → ready, failed → error
  let status: 'loading' | 'ready' | 'error' | undefined;
  if (s.status === 'pending' || s.status === 'processing') {
    status = 'loading';
  } else if (s.status === 'ready') {
    status = 'ready';
  } else if (s.status === 'failed') {
    status = 'error';
  }

  return {
    id: String(s.id),
    name: s.name,
    type: s.type === 'note' ? 'youdao' : s.type,
    size: s.file_size || undefined,
    url: s.original_url || undefined,
    selected: s.vectorized, // 只有已入库的默认选中
    status,
    errorMessage: s.error_message || undefined,
    vectorized: s.vectorized,
    createdAt: s.created_at,
    updatedAt: s.updated_at,
  };
}

export const useNotebookStore = create<NotebookState>((set, get) => ({
  notebooks: [],
  currentNotebookId: null,
  currentConversationId: null,
  taskIdBySourceId: {},
  loading: false,
  streamingContent: '',

  fetchNotebooks: async () => {
    set({ loading: true });
    try {
      const res = await notebookApi.listNotebooks();
      if (res.code === 0) {
        const notebooks: Notebook[] = res.data.map((nb) => ({
          id: String(nb.id),
          name: nb.name,
          sources: [],
          conversations: [],
          notes: [],
          createdAt: nb.created_at,
          updatedAt: nb.updated_at,
        }));
        set({ notebooks });
      }
    } catch (err) {
      console.error('Failed to fetch notebooks:', err);
    } finally {
      set({ loading: false });
    }
  },

  getCurrentNotebook: () => {
    const { notebooks, currentNotebookId } = get();
    return notebooks.find((n) => n.id === currentNotebookId);
  },

  getCurrentConversation: () => {
    const notebook = get().getCurrentNotebook();
    if (!notebook) return undefined;
    const { currentConversationId } = get();
    return notebook.conversations.find((c) => c.id === currentConversationId);
  },

  getSelectedSources: () => {
    const notebook = get().getCurrentNotebook();
    if (!notebook) return [];
    return notebook.sources.filter((s) => s.selected);
  },

  setCurrentNotebook: async (id) => {
    set({ currentNotebookId: id, currentConversationId: null });

    // Fetch sources and conversations
    await Promise.all([
      get().fetchSources(id),
      get().fetchConversations(id),
    ]);

    // After fetching, restore last used conversation or select the latest one
    const notebook = get().notebooks.find((n) => n.id === id);
    if (notebook) {
      if (notebook.conversations.length > 0) {
        // Try to restore the last used conversation from localStorage
        const lastConversationId = localStorage.getItem(`lastConversation_${id}`);
        const lastConversation = lastConversationId
          ? notebook.conversations.find((c) => c.id === lastConversationId)
          : null;

        if (lastConversation) {
          // Restore the last used conversation
          set({ currentConversationId: lastConversation.id });
        } else {
          // Fallback to the latest conversation (first in list since sorted by updated_at desc)
          set({ currentConversationId: notebook.conversations[0].id });
          // Save this as the new last conversation
          localStorage.setItem(`lastConversation_${id}`, notebook.conversations[0].id);
        }
      } else {
        // No conversations, create a new one
        await get().createConversation(id);
      }
    }
  },

  createNotebook: async (name) => {
    try {
      const res = await notebookApi.createNotebook(name);
      if (res.code === 0) {
        const newNotebook: Notebook = {
          id: String(res.data.id),
          name: res.data.name,
          sources: [],
          conversations: [],
          notes: [],
          createdAt: res.data.created_at,
          updatedAt: res.data.updated_at,
        };
        set((state) => ({
          notebooks: [newNotebook, ...state.notebooks],
          currentNotebookId: newNotebook.id,
        }));
      } else {
        throw new Error(res.message);
      }
    } catch (err: any) {
      if (err?.response?.status === 409) {
        throw new Error(err.response.data?.message || '已存在同名笔记本');
      }
      throw err;
    }
  },

  deleteNotebook: async (id) => {
    try {
      const res = await notebookApi.deleteNotebook(Number(id));
      if (res.code === 0) {
        set((state) => {
          const filtered = state.notebooks.filter((n) => n.id !== id);
          return {
            notebooks: filtered,
            currentNotebookId:
              state.currentNotebookId === id ? (filtered[0]?.id ?? null) : state.currentNotebookId,
          };
        });
      }
    } catch (err) {
      console.error('Failed to delete notebook:', err);
      throw err;
    }
  },

  renameNotebook: async (id, name) => {
    try {
      const res = await notebookApi.renameNotebook(Number(id), name);
      if (res.code === 0) {
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === id ? { ...n, name, updatedAt: new Date().toISOString() } : n
          ),
        }));
      } else {
        throw new Error(res.message);
      }
    } catch (err: any) {
      if (err?.response?.status === 409) {
        throw new Error(err.response.data?.message || '已存在同名笔记本');
      }
      throw err;
    }
  },

  // ---- Source actions (API-backed) ----

  fetchSources: async (notebookId, page = 1, size = 50, keyword) => {
    try {
      const res = await sourceApi.listSources(Number(notebookId), { page, size, keyword });
      if (res.code === 0) {
        set((state) => {
          const notebook = state.notebooks.find(n => n.id === notebookId);
          if (!notebook) return state;

          // 保留已有的选中状态（source ID → selected 映射）
          const selectedMap = new Map(notebook.sources.map(s => [s.id, s.selected]));

          const serverSources = res.data.list.map((s) => {
            const source = toSource(s);
            // 如果之前有该 source 的选中状态，则保留；否则默认为 true
            source.selected = selectedMap.has(source.id) ? selectedMap.get(source.id)! : true;
            return source;
          });

          // 保留本地的 loading/error 状态的 placeholder（ID 以 loading- 开头）
          const localPlaceholders = notebook.sources.filter(s =>
            s.id.startsWith('loading-')
          );

          // 合并：服务器数据 + 本地 placeholder
          const mergedSources = [...serverSources, ...localPlaceholders];

          return {
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId ? { ...n, sources: mergedSources } : n
            ),
          };
        });
      }
    } catch (err) {
      console.error('Failed to fetch sources:', err);
    }
  },

  addSource: (notebookId, source) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: [...n.sources, source], updatedAt: new Date().toISOString() }
          : n
      ),
    }));
  },

  updateSource: (notebookId, sourceId, updates) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? {
            ...n,
            sources: n.sources.map((s) =>
              s.id === sourceId
                ? { ...s, ...updates, ...(updates.status === 'error' ? { selected: false } : {}) }
                : s
            ),
          }
          : n
      ),
    }));
  },

  removeSource: async (notebookId, sourceId) => {
    // loading 状态的 source 是本地 placeholder（ID 以 loading- 开头）
    if (sourceId.startsWith('loading-')) {
      // 查找 placeholder 关联的后端任务 ID，通知后端取消
      const notebook = get().notebooks.find(n => n.id === notebookId);
      const placeholder = notebook?.sources.find(s => s.id === sourceId);
      if (placeholder?.taskId) {
        try {
          await importApi.deleteImportTask(placeholder.taskId);
        } catch {
          // 任务可能已完成或不存在，忽略错误
        }
      }
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? { ...n, sources: n.sources.filter((s) => s.id !== sourceId) }
            : n
        ),
      }));
      return;
    }

    try {
      const res = await sourceApi.deleteSource(Number(notebookId), Number(sourceId));
      if (res.code === 0) {
        // 检查该 source 是否关联了正在运行的导入任务，有则取消
        const taskId = get().taskIdBySourceId[sourceId];
        if (taskId) {
          try {
            await importApi.deleteImportTask(taskId);
          } catch {
            // 任务可能已完成或不存在，忽略错误
          }
          // 清理映射
          set((state) => {
            const newMapping = { ...state.taskIdBySourceId };
            delete newMapping[sourceId];
            return { taskIdBySourceId: newMapping };
          });
        }

        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, sources: n.sources.filter((s) => s.id !== sourceId) }
              : n
          ),
        }));
      }
    } catch (err) {
      console.error('Failed to delete source:', err);
      throw err;
    }
  },

  batchRemoveSources: async (notebookId, sourceIds) => {
    try {
      // 找出 loading 状态的 placeholder，通知后端取消关联的导入任务
      const notebook = get().notebooks.find(n => n.id === notebookId);
      const loadingIds = sourceIds.filter(id => id.startsWith('loading-'));
      for (const loadingId of loadingIds) {
        const placeholder = notebook?.sources.find(s => s.id === loadingId);
        if (placeholder?.taskId) {
          try {
            await importApi.deleteImportTask(placeholder.taskId);
          } catch {
            // 任务可能已完成或不存在，忽略错误
          }
        }
      }

      // 找出关联了导入任务的真实 source，通知后端取消
      const taskIdsToCancel = new Set<string>();
      for (const sourceId of sourceIds) {
        const taskId = get().taskIdBySourceId[sourceId];
        if (taskId) taskIdsToCancel.add(taskId);
      }
      for (const taskId of taskIdsToCancel) {
        try {
          await importApi.deleteImportTask(taskId);
        } catch {
          // 忽略
        }
      }

      // 过滤出有效的数字 ID（排除 loading-xxx 等临时 ID）
      const validIds = sourceIds
        .filter(id => !id.startsWith('loading-'))
        .map(Number)
        .filter(id => !isNaN(id) && id > 0);

      // 如果有有效的 ID，调用批量删除 API
      if (validIds.length > 0) {
        const res = await sourceApi.batchDeleteSources(Number(notebookId), validIds);
        if (res.code !== 0) {
          throw new Error(res.message);
        }
      }

      // 清理映射
      set((state) => {
        const newMapping = { ...state.taskIdBySourceId };
        for (const sourceId of sourceIds) delete newMapping[sourceId];
        return { taskIdBySourceId: newMapping };
      });

      // 从本地状态中移除所有选中的 source（包括临时 placeholder）
      const idSet = new Set(sourceIds);
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? { ...n, sources: n.sources.filter((s) => !idSet.has(s.id)) }
            : n
        ),
      }));
    } catch (err) {
      console.error('Failed to batch delete sources:', err);
      throw err;
    }
  },

  deleteFailedSources: async (notebookId) => {
    try {
      const res = await sourceApi.deleteFailedSources(Number(notebookId));
      if (res.code === 0) {
        const deletedCount = res.data.deleted_count;

        // 从本地状态中移除所有 failed 状态的 source
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, sources: n.sources.filter((s) => s.status !== 'error') }
              : n
          ),
        }));

        return deletedCount;
      }
      throw new Error(res.message);
    } catch (err) {
      console.error('Failed to delete failed sources:', err);
      throw err;
    }
  },

  toggleSourceSelection: (notebookId, sourceId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: n.sources.map((s) => s.id === sourceId ? { ...s, selected: !s.selected } : s) }
          : n
      ),
    }));
  },

  renameSource: async (notebookId, sourceId, name) => {
    try {
      const res = await sourceApi.renameSource(Number(notebookId), Number(sourceId), name);
      if (res.code === 0) {
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, sources: n.sources.map((s) => s.id === sourceId ? { ...s, name } : s) }
              : n
          ),
        }));
      }
    } catch (err) {
      console.error('Failed to rename source:', err);
      throw err;
    }
  },

  selectAllSources: (notebookId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: n.sources.map((s) => ({ ...s, selected: true })) }
          : n
      ),
    }));
  },

  deselectAllSources: (notebookId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: n.sources.map((s) => ({ ...s, selected: false })) }
          : n
      ),
    }));
  },

  fetchSourceContent: async (notebookId, sourceId) => {
    const res = await sourceApi.getSourceContent(Number(notebookId), Number(sourceId));
    if (res.code === 0) return res.data.content;
    throw new Error(res.message);
  },

  fetchSourceOriginal: async (notebookId, sourceId) => {
    const res = await sourceApi.getSourceOriginal(Number(notebookId), Number(sourceId));
    if (res.code === 0) return { content: res.data.content, type: res.data.type };
    throw new Error(res.message);
  },

  getSourceDownloadURL: async (notebookId, sourceId) => {
    const res = await sourceApi.getSourceDownloadURL(Number(notebookId), Number(sourceId));
    if (res.code === 0) return res.data.url;
    throw new Error(res.message);
  },

  // ---- Import actions ----

  importFile: async (notebookId, file) => {
    // Optimistic: add a loading placeholder immediately
    const tempId = `loading-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const placeholder: Source = {
      id: tempId,
      name: file.name,
      type: 'file',
      size: file.size,
      selected: true,
      status: 'loading',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    get().addSource(notebookId, placeholder);

    try {
      const res = await importApi.importFile(Number(notebookId), file);
      if (res.code === 0) {
        const source = toSource(res.data);
        // 后端立即返回 processing 状态的 source，替换 placeholder
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, sources: [...n.sources.filter((s) => s.id !== tempId), source] }
              : n
          ),
        }));

        // 后台轮询 source 状态，直到处理完成
        if (source.status === 'loading') {
          const pollSource = async () => {
            const maxAttempts = 120;
            for (let i = 0; i < maxAttempts; i++) {
              await new Promise(r => setTimeout(r, 2000));
              try {
                await get().fetchSources(notebookId);
                const nb = get().notebooks.find(n => n.id === notebookId);
                const src = nb?.sources.find(s => s.id === source.id);
                if (!src) return; // source 已被删除
                if (src.status !== 'loading') return; // 处理完成
              } catch {
                return;
              }
            }
          };
          pollSource();
        }

        return source;
      }
      // API returned error code
      get().updateSource(notebookId, tempId, {
        status: 'error',
        errorMessage: res.message || '导入失败',
      });
      throw new Error(res.message);
    } catch (err: any) {
      // Mark placeholder as error (only if not already marked)
      const currentNotebook = get().notebooks.find(n => n.id === notebookId);
      const placeholderSource = currentNotebook?.sources.find(s => s.id === tempId);
      if (placeholderSource?.status !== 'error') {
        get().updateSource(notebookId, tempId, {
          status: 'error',
          errorMessage: getErrorMessage(err, '导入失败'),
        });
      }
      throw err;
    }
  },

  previewAudio: async (notebookId, file) => {
    // Optimistic: add a loading placeholder immediately
    const tempId = `loading-audio-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const placeholder: Source = {
      id: tempId,
      name: file.name,
      type: 'audio',
      size: file.size,
      selected: true,
      status: 'loading',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    get().addSource(notebookId, placeholder);

    try {
      const res = await importApi.previewAudio(Number(notebookId), file);
      if (res.code === 0) {
        // 上传成功，更新 placeholder 状态为 loading（等待转写完成）
        get().updateSource(notebookId, tempId, {
          previewId: res.data.preview_id,
        });
        // 后台轮询转写状态
        get().pollAudioPreview(res.data.preview_id).then((preview) => {
          get().updateSource(notebookId, tempId, {
            status: 'ready',
            content: preview.transcribed_text,
            previewId: preview.preview_id,
          });
        }).catch((err: any) => {
          get().updateSource(notebookId, tempId, {
            status: 'error',
            errorMessage: getErrorMessage(err, '音频转写失败'),
          });
        });
        return res.data;
      }
      // API returned error code
      get().updateSource(notebookId, tempId, {
        status: 'error',
        errorMessage: res.message || '音频转写失败',
      });
      throw new Error(res.message);
    } catch (err: any) {
      // Mark placeholder as error (only if not already marked)
      const currentNotebook = get().notebooks.find(n => n.id === notebookId);
      const placeholderSource = currentNotebook?.sources.find(s => s.id === tempId);
      if (placeholderSource?.status !== 'error') {
        get().updateSource(notebookId, tempId, {
          status: 'error',
          errorMessage: getErrorMessage(err, '音频转写失败'),
        });
      }
      throw err;
    }
  },

  pollAudioPreview: async (previewId) => {
    const maxAttempts = 120; // 最多轮询 120 次（约 4 分钟）
    const interval = 2000; // 每 2 秒轮询一次

    for (let i = 0; i < maxAttempts; i++) {
      const res = await importApi.getAudioPreviewStatus(previewId);
      if (res.code !== 0) {
        throw new Error(res.message || '查询转写状态失败');
      }
      const data = res.data;
      if (data.status === 'ready') {
        return data;
      }
      if (data.status === 'failed') {
        throw new Error(data.error_msg || '音频转写失败');
      }
      if (data.status === 'confirmed') {
        throw new Error('该音频已被确认过');
      }
      // pending 或 processing，继续轮询
      await new Promise((resolve) => setTimeout(resolve, interval));
    }
    throw new Error('音频转写超时，请稍后重试');
  },

  confirmAudio: async (previewId, notebookId, content) => {
    const res = await importApi.confirmAudio({
      preview_id: previewId,
      content: content || undefined,
      notebook_id: Number(notebookId),
    });
    if (res.code === 0) {
      const source = toSource(res.data);
      // Remove the pending placeholder (has previewId) and add the confirmed source
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? {
              ...n,
              sources: [
                ...n.sources.filter((s) => s.previewId !== previewId),
                source
              ]
            }
            : n
        ),
      }));
      return source;
    }
    throw new Error(res.message);
  },

  getImportTask: async (taskId) => {
    const res = await importApi.getImportTask(taskId);
    if (res.code === 0) return res.data;
    throw new Error(res.message);
  },

  deleteImportTask: async (taskId) => {
    const res = await importApi.deleteImportTask(taskId);
    if (res.code !== 0) throw new Error(res.message);
  },

  // ---- Search actions ----

  searchSources: async (notebookId, query) => {
    const res = await searchApi.search(Number(notebookId), query);
    if (res.code === 0) return res.data;
    throw new Error(res.message);
  },

  searchSourcesStream: async (notebookId, query, onEvent, signal) => {
    await searchApi.searchStream(Number(notebookId), query, onEvent, signal);
  },

  importFromURL: async (notebookId, url) => {
    // 调用后端创建 pending 状态的 Source
    const res = await searchApi.importFromURL(Number(notebookId), url);
    if (res.code !== 0) throw new Error(res.message);

    const { task_id: taskId, source_id: sourceId } = res.data;

    // 记录 sourceID → taskID 映射，以便取消时使用
    set((state) => ({
      taskIdBySourceId: { ...state.taskIdBySourceId, [String(sourceId)]: taskId },
    }));

    // 刷新列表，后端创建的 pending source 会出现在列表中
    await get().fetchSources(notebookId);

    // 轮询该 source 的状态，直到处理完成
    const pollSource = async () => {
      const maxAttempts = 60;
      for (let i = 0; i < maxAttempts; i++) {
        await new Promise(r => setTimeout(r, 2000));
        try {
          await get().fetchSources(notebookId);
          const nb = get().notebooks.find(n => n.id === notebookId);
          const src = nb?.sources.find(s => s.id === String(sourceId));
          if (!src) return; // source 已被删除
          if (src.status !== 'loading') return; // 处理完成（ready 或 error）
        } catch {
          return;
        }
      }
    };
    pollSource();

    return { taskId, sourceId };
  },

  importSearchResults: async (notebookId, items) => {
    const res = await searchApi.importSearchResults(Number(notebookId), items);
    if (res.code !== 0) throw new Error(res.message);

    const { task_id: taskId, source_ids: sourceIds } = res.data;

    // 记录 sourceID → taskID 映射，以便删除 source 时取消任务
    const mapping: Record<string, string> = {};
    for (const sid of sourceIds) {
      mapping[String(sid)] = taskId;
    }
    set((state) => ({ taskIdBySourceId: { ...state.taskIdBySourceId, ...mapping } }));

    // 后端已创建 pending 状态的 Source，立即刷新列表显示它们
    await get().fetchSources(notebookId);

    // 轮询这些 Source 的状态，直到全部处理完成
    if (sourceIds.length > 0) {
      const pollSources = async () => {
        const maxAttempts = 120;
        for (let i = 0; i < maxAttempts; i++) {
          await new Promise(r => setTimeout(r, 2000));
          try {
            // 刷新 sources 列表获取最新状态
            await get().fetchSources(notebookId);

            // 检查这些 source 是否还有 pending/processing 状态的
            const notebook = get().notebooks.find(n => n.id === notebookId);
            if (!notebook) return;
            const pendingCount = notebook.sources.filter(
              s => sourceIds.includes(Number(s.id)) && (s.status === 'loading')
            ).length;
            if (pendingCount === 0) {
              // 全部处理完成，清理映射
              set((state) => {
                const newMapping = { ...state.taskIdBySourceId };
                for (const sid of sourceIds) delete newMapping[String(sid)];
                return { taskIdBySourceId: newMapping };
              });
              return;
            }
          } catch {
            return;
          }
        }
      };
      pollSources();
    }

    return { taskId, sourceIds };
  },

  // ---- Conversation actions (API-backed) ----

  fetchConversations: async (notebookId) => {
    try {
      const res = await chatApi.listConversations(Number(notebookId));
      if (res.code === 0) {
        const conversations: Conversation[] = res.data.map((conv) => ({
          id: String(conv.id),
          title: conv.title,
          notebookId: String(conv.notebook_id),
          messages: [],
          createdAt: conv.created_at,
          updatedAt: conv.updated_at,
        }));
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId ? { ...n, conversations } : n
          ),
        }));
      }
    } catch (err) {
      console.error('Failed to fetch conversations:', err);
    }
  },

  createConversation: async (notebookId, title) => {
    try {
      const res = await chatApi.createConversation(Number(notebookId), title);
      if (res.code === 0) {
        const newConv: Conversation = {
          id: String(res.data.id),
          title: title || '新对话',
          notebookId,
          messages: [],
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };
        // Save as the last used conversation
        localStorage.setItem(`lastConversation_${notebookId}`, newConv.id);

        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, conversations: [newConv, ...n.conversations] }
              : n
          ),
          currentConversationId: newConv.id,
        }));
        return newConv.id;
      }
      throw new Error(res.message);
    } catch (err) {
      console.error('Failed to create conversation:', err);
      throw err;
    }
  },

  setCurrentConversation: (id) => {
    const notebookId = get().currentNotebookId;
    if (notebookId) {
      localStorage.setItem(`lastConversation_${notebookId}`, id);
    }
    set({ currentConversationId: id });
  },

  deleteConversation: async (notebookId, conversationId) => {
    try {
      const res = await chatApi.deleteConversation(Number(conversationId));
      if (res.code === 0) {
        // Clean up localStorage if this was the last used conversation
        const lastConversationId = localStorage.getItem(`lastConversation_${notebookId}`);
        if (lastConversationId === conversationId) {
          localStorage.removeItem(`lastConversation_${notebookId}`);
        }

        set((state) => {
          const notebook = state.notebooks.find((n) => n.id === notebookId);
          if (!notebook) return state;
          const filtered = notebook.conversations.filter((c) => c.id !== conversationId);

          // If we need to switch to a new conversation, save it as the last used
          if (state.currentConversationId === conversationId && filtered.length > 0) {
            localStorage.setItem(`lastConversation_${notebookId}`, filtered[0].id);
          }

          return {
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId ? { ...n, conversations: filtered } : n
            ),
            currentConversationId:
              state.currentConversationId === conversationId
                ? (filtered[0]?.id ?? null)
                : state.currentConversationId,
          };
        });
      }
    } catch (err) {
      console.error('Failed to delete conversation:', err);
      throw err;
    }
  },

  renameConversation: async (notebookId, conversationId, title) => {
    try {
      const res = await chatApi.updateConversation(Number(conversationId), title);
      if (res.code === 0) {
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? {
                  ...n,
                  conversations: n.conversations.map((c) =>
                    c.id === conversationId ? { ...c, title, updatedAt: new Date().toISOString() } : c
                  ),
                }
              : n
          ),
        }));
      }
    } catch (err) {
      console.error('Failed to rename conversation:', err);
      throw err;
    }
  },

  // ---- Message actions (API-backed) ----

  fetchMessages: async (notebookId, conversationId) => {
    try {
      const res = await chatApi.getMessages(Number(conversationId));
      if (res.code === 0) {
        const messages: ChatMessage[] = res.data.map((msg) => ({
          id: String(msg.id),
          role: msg.role,
          content: msg.content,
          timestamp: msg.created_at,
          references: msg.metadata?.references?.map((ref) => ({
            sourceId: String(ref.source_id),
            sourceName: ref.source_name,
            parentBlockId: ref.parent_block_id,
            chunkContent: ref.chunk_content,
            score: ref.score,
          })),
        }));
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? {
                  ...n,
                  conversations: n.conversations.map((c) =>
                    c.id === conversationId ? { ...c, messages } : c
                  ),
                }
              : n
          ),
        }));
      }
    } catch (err) {
      console.error('Failed to fetch messages:', err);
    }
  },

  sendMessage: async (notebookId, conversationId, content, sourceIds, llmConfigId) => {
    // Add user message immediately
    const userMessageId = `msg-user-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const userMessage: ChatMessage = {
      id: userMessageId,
      role: 'user',
      content,
      timestamp: new Date().toISOString(),
    };
    console.log('Adding user message:', userMessageId, 'to conversation:', conversationId);
    get().addMessage(notebookId, conversationId, userMessage);

    // Add streaming assistant message placeholder
    const assistantMessageId = `msg-assistant-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const assistantMessage: ChatMessage = {
      id: assistantMessageId,
      role: 'assistant',
      content: '',
      timestamp: new Date().toISOString(),
      isStreaming: true,
    };
    console.log('Adding assistant placeholder:', assistantMessageId, 'to conversation:', conversationId);
    get().addMessage(notebookId, conversationId, assistantMessage);

    try {
      console.log('Sending message to conversation:', conversationId);
      const response = await chatApi.sendMessage(
        Number(conversationId),
        content,
        sourceIds,
        llmConfigId
      );

      console.log('Response status:', response.status, response.ok);
      console.log('Response headers:', Object.fromEntries(response.headers.entries()));

      if (!response.ok) {
        const errorText = await response.text();
        console.error('Response error:', errorText);
        throw new Error(getChatErrorMessage(errorText));
      }

      // Check if response is SSE or regular JSON
      const contentType = response.headers.get('content-type');
      console.log('Content-Type:', contentType);

      if (contentType && contentType.includes('application/json')) {
        // Regular JSON response - handle it directly
        const jsonData = await response.json();
        console.log('JSON response:', jsonData);

        if (jsonData.code === 0 && jsonData.data) {
          // Update the assistant message with the response
          const assistantContent = jsonData.data.content || jsonData.data.message || '';
          set((state) => ({
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId
                ? {
                    ...n,
                    conversations: n.conversations.map((c) =>
                      c.id === conversationId
                        ? {
                            ...c,
                            messages: c.messages.map((m) =>
                              m.id === assistantMessageId
                                ? { ...m, content: assistantContent, isStreaming: false }
                                : m
                            ),
                          }
                        : c
                    ),
                  }
                : n
            ),
          }));
        } else {
          throw new Error(jsonData.message || '请求失败');
        }
        return;
      }

      // SSE response - parse the stream
      let accumulatedContent = '';
      const abortController = chatApi.parseSSEStream(response, {
        onToken: (token) => {
          console.log('Token received:', token);
          accumulatedContent += token;

          // Update both streamingContent (for immediate display) and notebooks
          set((state) => {
            const newNotebooks = state.notebooks.map((n) => {
              if (n.id !== notebookId) return n;
              return {
                ...n,
                conversations: n.conversations.map((c) => {
                  if (c.id !== conversationId) return c;
                  return {
                    ...c,
                    messages: c.messages.map((m) => {
                      if (m.id !== assistantMessageId) return m;
                      return { ...m, content: accumulatedContent };
                    }),
                    updatedAt: new Date().toISOString(),
                  };
                }),
              };
            });
            return { notebooks: newNotebooks, streamingContent: accumulatedContent };
          });
        },
        onReference: (references) => {
          console.log('References received:', references);
          const refs: Reference[] = references.map((ref) => ({
            sourceId: String(ref.source_id),
            sourceName: ref.source_name,
            parentBlockId: ref.parent_block_id,
            chunkContent: ref.chunk_content,
            score: ref.score,
          }));
          set((state) => ({
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId
                ? {
                    ...n,
                    conversations: n.conversations.map((c) =>
                      c.id === conversationId
                        ? {
                            ...c,
                            messages: c.messages.map((m) =>
                              m.id === assistantMessageId
                                ? { ...m, references: refs }
                                : m
                            ),
                          }
                        : c
                    ),
                  }
                : n
            ),
          }));
        },
        onTitle: (title) => {
          console.log('Title received:', title);
          // Update conversation title from SSE event
          set((state) => ({
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId
                ? {
                    ...n,
                    conversations: n.conversations.map((c) =>
                      c.id === conversationId ? { ...c, title } : c
                    ),
                  }
                : n
            ),
          }));
        },
        onDone: () => {
          console.log('Stream completed');
          set((state) => ({
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId
                ? {
                    ...n,
                    conversations: n.conversations.map((c) =>
                      c.id === conversationId
                        ? {
                            ...c,
                            messages: c.messages.map((m) =>
                              m.id === assistantMessageId
                                ? { ...m, isStreaming: false }
                                : m
                            ),
                            updatedAt: new Date().toISOString(),
                          }
                        : c
                    ),
                  }
                : n
            ),
          }));
        },
        onError: (error) => {
          console.error('Stream error:', error);
          const friendlyMessage = getChatErrorMessage(error);
          set((state) => ({
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId
                ? {
                    ...n,
                    conversations: n.conversations.map((c) =>
                      c.id === conversationId
                        ? {
                            ...c,
                            messages: c.messages.map((m) =>
                              m.id === assistantMessageId
                                ? { ...m, content: friendlyMessage, isStreaming: false }
                                : m
                            ),
                          }
                        : c
                    ),
                  }
                : n
            ),
          }));
        },
      });
      // Save abort controller for stopGeneration to use
      currentStreamAbortController = abortController;
    } catch (err) {
      console.error('Failed to send message:', err);
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? {
                ...n,
                conversations: n.conversations.map((c) =>
                  c.id === conversationId
                    ? {
                        ...c,
                        messages: c.messages.map((m) =>
                          m.id === assistantMessageId
                            ? { ...m, content: '发送消息失败，请重试', isStreaming: false }
                            : m
                        ),
                      }
                    : c
                ),
              }
            : n
        ),
      }));
    }
  },

  stopGeneration: async (notebookId, conversationId) => {
    try {
      // Abort the frontend SSE stream first
      if (currentStreamAbortController) {
        currentStreamAbortController.abort();
        currentStreamAbortController = null;
      }
      await chatApi.stopGeneration(Number(conversationId));
      // Mark any streaming messages as done
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? {
                ...n,
                conversations: n.conversations.map((c) =>
                  c.id === conversationId
                    ? {
                        ...c,
                        messages: c.messages.map((m) =>
                          m.isStreaming ? { ...m, isStreaming: false } : m
                        ),
                      }
                    : c
                ),
              }
            : n
        ),
      }));
    } catch (err) {
      console.error('Failed to stop generation:', err);
    }
  },

  addMessage: (notebookId, conversationId, message) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? {
              ...n,
              conversations: n.conversations.map((c) =>
                c.id === conversationId
                  ? {
                      ...c,
                      messages: [...c.messages, message],
                      updatedAt: new Date().toISOString(),
                    }
                  : c
              ),
            }
          : n
      ),
    }));
  },

  updateMessage: (notebookId, conversationId, messageId, updates) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? {
              ...n,
              conversations: n.conversations.map((c) =>
                c.id === conversationId
                  ? {
                      ...c,
                      messages: c.messages.map((m) =>
                        m.id === messageId ? { ...m, ...updates } : m
                      ),
                    }
                  : c
              ),
            }
          : n
      ),
    }));
  },

  addNote: (notebookId, note) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: [note, ...n.notes], updatedAt: new Date().toISOString() }
          : n
      ),
    }));
  },

  deleteNote: (notebookId, noteId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.filter((note) => note.id !== noteId) }
          : n
      ),
    }));
  },

  renameNote: (notebookId, noteId, title) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.map((note) => note.id === noteId ? { ...note, title } : note) }
          : n
      ),
    }));
  },

  updateNoteContent: (notebookId, noteId, content) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.map((note) => note.id === noteId ? { ...note, content, updatedAt: new Date().toISOString() } : note) }
          : n
      ),
    }));
  },

  toggleNoteSource: (notebookId, noteId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.map((note) => note.id === noteId ? { ...note, isSource: !note.isSource } : note) }
          : n
      ),
    }));
  },

  reimportAll: async () => {
    const res = await sourceApi.reimportAll();
    if (res.code === 0) {
      // 刷新所有笔记本的 sources
      const { notebooks } = get();
      for (const notebook of notebooks) {
        await get().fetchSources(notebook.id);
      }
      return res.data.reimported_count;
    }
    throw new Error(res.message || '重新导入失败');
  },

  reimportSelected: async (sourceIds: string[]) => {
    const numericIds = sourceIds.map(id => Number(id));
    const res = await sourceApi.reimportSources(numericIds);
    if (res.code === 0) {
      // 刷新当前笔记本的 sources
      const { currentNotebookId } = get();
      if (currentNotebookId) {
        await get().fetchSources(currentNotebookId);
      }
      return res.data.reimported_count;
    }
    throw new Error(res.message || '重新导入失败');
  },
}));
