import { useState, useRef, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Send, Plus, MessageSquare, Trash2, Save, Loader2,
  ChevronDown, Sparkles, Edit3, Square, X
} from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useNotebookStore } from '../../stores/useNotebookStore';
import { cn } from '../../utils/cn';
import type { NoteType, Reference } from '../../types';

// Reference popover component
function ReferencePopover({ references }: { references: Reference[] }) {
  const [openIndex, setOpenIndex] = useState<number | null>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setOpenIndex(null);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <span className="inline-flex gap-1 align-super text-xs">
      {references.map((ref, idx) => (
        <span key={idx} className="relative">
          <button
            onClick={(e) => {
              e.stopPropagation();
              setOpenIndex(openIndex === idx ? null : idx);
            }}
            className="inline-flex items-center justify-center w-4 h-4 rounded text-[10px] font-bold bg-accent/20 text-accent hover:bg-accent/30 transition-colors cursor-pointer"
          >
            {idx + 1}
          </button>
          {openIndex === idx && (
            <div
              ref={popoverRef}
              className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 w-72 bg-bg-card border border-border-light rounded-xl shadow-xl z-50 overflow-hidden"
            >
              <div className="flex items-center justify-between px-3 py-2 border-b border-border bg-bg-secondary/50">
                <span className="text-xs font-medium text-accent truncate">{ref.sourceName}</span>
                <button
                  onClick={() => setOpenIndex(null)}
                  className="p-0.5 hover:bg-bg-hover rounded cursor-pointer"
                >
                  <X size={12} className="text-text-muted" />
                </button>
              </div>
              <div className="px-3 py-2 max-h-48 overflow-y-auto">
                <div className="prose prose-xs prose-invert max-w-none text-xs">
                  <Markdown remarkPlugins={[remarkGfm]}>{ref.chunkContent}</Markdown>
                </div>
              </div>
              <div className="px-3 py-1.5 border-t border-border bg-bg-secondary/30">
                <span className="text-[10px] text-text-muted">
                  相关度: {(ref.score * 100).toFixed(0)}%
                </span>
              </div>
            </div>
          )}
        </span>
      ))}
    </span>
  );
}

// Custom markdown component with reference support
function MarkdownWithReferences({
  content,
  references
}: {
  content: string;
  references?: Reference[];
}) {
  if (!content) {
    return null;
  }

  if (!references || references.length === 0) {
    return (
      <div className="prose prose-sm prose-invert max-w-none">
        <Markdown remarkPlugins={[remarkGfm]}>{content}</Markdown>
      </div>
    );
  }

  // Split content by reference markers like [1], [2], etc.
  const parts = content.split(/(\[\d+\])/g);

  return (
    <div className="prose prose-sm prose-invert max-w-none">
      {parts.map((part, i) => {
        if (!part) return null;
        const refMatch = part.match(/^\[(\d+)\]$/);
        if (refMatch) {
          const refIdx = parseInt(refMatch[1]) - 1;
          if (refIdx >= 0 && refIdx < references.length) {
            return (
              <ReferencePopover
                key={i}
                references={[references[refIdx]]}
              />
            );
          }
        }
        return (
          <span key={i} className="inline">
            <Markdown remarkPlugins={[remarkGfm]}>{part}</Markdown>
          </span>
        );
      })}
    </div>
  );
}

export default function ChatPanel() {
  const {
    currentNotebookId, currentConversationId,
    notebooks, streamingContent, createConversation, setCurrentConversation, deleteConversation,
    renameConversation, sendMessage, stopGeneration, fetchMessages, addNote
  } = useNotebookStore();

  const notebook = notebooks.find((n) => n.id === currentNotebookId);
  const conversation = notebook?.conversations.find((c) => c.id === currentConversationId);

  const [input, setInput] = useState('');
  const [showConvList, setShowConvList] = useState(false);
  const [editingConvId, setEditingConvId] = useState<string | null>(null);
  const [editConvTitle, setEditConvTitle] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const prevConvIdRef = useRef<string | null>(null);

  // Check if any message is streaming
  const isStreaming = conversation?.messages.some((m) => m.isStreaming) ?? false;

  // Get the streaming message content for display
  const streamingMessage = conversation?.messages.find((m) => m.isStreaming);
  const displayContent = streamingMessage?.content || streamingContent;

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [conversation?.messages, displayContent]);

  // Fetch messages only when conversation ID changes (not on every render)
  useEffect(() => {
    if (currentNotebookId && conversation?.id && conversation.id !== prevConvIdRef.current) {
      prevConvIdRef.current = conversation.id;
      fetchMessages(currentNotebookId, conversation.id);
    }
  }, [currentNotebookId, conversation?.id, fetchMessages]);

  if (!notebook || !currentNotebookId) return null;

  const selectedSources = notebook.sources.filter((s) => s.selected && s.status !== 'error');

  const handleSend = async () => {
    if (!input.trim() || isStreaming || !conversation?.id) return;

    const messageContent = input.trim();
    setInput('');

    const sourceIds = selectedSources.map((s) => Number(s.id));

    try {
      await sendMessage(currentNotebookId, conversation.id, messageContent, sourceIds);
    } catch (err) {
      console.error('Failed to send message:', err);
    }
  };

  const handleStop = async () => {
    if (!conversation?.id || !currentNotebookId) return;
    await stopGeneration(currentNotebookId, conversation.id);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleSaveAsNote = (content: string) => {
    const note = {
      id: `note-${Date.now()}`,
      title: content.slice(0, 20).replace(/[#*\n]/g, ''),
      type: 'note' as NoteType,
      content,
      isSource: false,
      notebookId: currentNotebookId,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    addNote(currentNotebookId, note);
  };

  const handleStartRenameConv = (id: string, title: string) => {
    setEditingConvId(id);
    setEditConvTitle(title);
  };

  const handleFinishRenameConv = async () => {
    if (editingConvId && editConvTitle.trim()) {
      try {
        await renameConversation(currentNotebookId, editingConvId, editConvTitle.trim());
      } catch (err) {
        console.error('Failed to rename conversation:', err);
      }
    }
    setEditingConvId(null);
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header with conversation switcher */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border flex-shrink-0">
        <div className="relative">
          <button
            onClick={() => setShowConvList(!showConvList)}
            className="flex items-center gap-2 text-sm font-medium text-text-primary hover:text-accent transition-colors cursor-pointer"
          >
            <MessageSquare size={15} />
            <span className="max-w-[200px] truncate">{conversation?.title || '新对话'}</span>
            <ChevronDown size={13} className={cn('transition-transform', showConvList && 'rotate-180')} />
          </button>

          <AnimatePresence>
            {showConvList && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setShowConvList(false)} />
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  className="absolute left-0 top-full mt-2 w-72 bg-bg-card border border-border-light rounded-xl shadow-xl z-50 max-h-80 overflow-y-auto"
                >
                  <div className="p-2">
                    <button
                      onClick={async () => {
                        try {
                          await createConversation(currentNotebookId);
                          setShowConvList(false);
                        } catch (err) {
                          console.error('Failed to create conversation:', err);
                        }
                      }}
                      className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-accent hover:bg-accent-glow transition-colors cursor-pointer"
                    >
                      <Plus size={13} /> 新建对话
                    </button>
                  </div>
                  <div className="border-t border-border px-2 py-1">
                    {notebook.conversations.map((conv) => (
                      <div
                        key={conv.id}
                        className={cn(
                          'group flex items-center gap-2 px-3 py-2 rounded-lg cursor-pointer transition-all',
                          conversation?.id === conv.id ? 'bg-accent/10 text-accent' : 'text-text-secondary hover:bg-bg-hover'
                        )}
                        onClick={() => {
                          setCurrentConversation(conv.id);
                          setShowConvList(false);
                        }}
                      >
                        <MessageSquare size={12} className="flex-shrink-0" />
                        {editingConvId === conv.id ? (
                          <input
                            autoFocus
                            value={editConvTitle}
                            onChange={(e) => setEditConvTitle(e.target.value)}
                            onBlur={handleFinishRenameConv}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') handleFinishRenameConv();
                              if (e.key === 'Escape') setEditingConvId(null);
                            }}
                            onClick={(e) => e.stopPropagation()}
                            className="flex-1 text-xs bg-transparent outline-none border-b border-accent"
                          />
                        ) : (
                          <span className="flex-1 text-xs truncate">{conv.title}</span>
                        )}
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            handleStartRenameConv(conv.id, conv.title);
                          }}
                          className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-bg-active transition-all cursor-pointer"
                        >
                          <Edit3 size={11} />
                        </button>
                        <button
                          onClick={async (e) => {
                            e.stopPropagation();
                            await deleteConversation(currentNotebookId, conv.id);
                          }}
                          className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-error/10 transition-all cursor-pointer"
                        >
                          <Trash2 size={11} className="text-error" />
                        </button>
                      </div>
                    ))}
                  </div>
                </motion.div>
              </>
            )}
          </AnimatePresence>
        </div>

        {/* Right side spacer for balance */}
        <div />
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-4">
        {(!conversation || conversation.messages.length === 0) && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="flex flex-col items-center justify-center h-full text-center"
          >
            <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-accent/20 to-teal/20 flex items-center justify-center mb-4">
              <Sparkles size={28} className="text-accent" />
            </div>
            <h3 className="text-base font-semibold text-text-primary mb-2">开始对话</h3>
            <p className="text-sm text-text-secondary max-w-xs">
              {selectedSources.length > 0
                ? `已选中 ${selectedSources.length} 份资料，输入问题开始对话`
                : '输入问题开始对话，或在左侧选择资料来源获得更精准的回答'}
            </p>
            <div className="flex flex-wrap gap-2 mt-4 justify-center">
              {['帮我总结核心观点', '生成思维导图', '出 10 道测验题'].map((q) => (
                <button
                  key={q}
                  onClick={() => setInput(q)}
                  className="px-3 py-1.5 rounded-full text-xs bg-bg-card border border-border-light text-text-secondary hover:text-accent hover:border-accent/30 transition-all cursor-pointer"
                >
                  {q}
                </button>
              ))}
            </div>
          </motion.div>
        )}

        {conversation?.messages.map((msg) => (
          <motion.div
            key={msg.id}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            className={cn('flex gap-3', msg.role === 'user' ? 'justify-end' : 'justify-start')}
          >
            {msg.role === 'assistant' && (
              <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-accent to-teal flex items-center justify-center flex-shrink-0 mt-0.5">
                <Sparkles size={13} className="text-white" />
              </div>
            )}
            <div
              className={cn(
                'max-w-[80%] rounded-2xl px-4 py-3 text-sm',
                msg.role === 'user'
                  ? 'bg-accent text-white rounded-br-md'
                  : 'bg-bg-card border border-border-light rounded-bl-md'
              )}
            >
              {msg.role === 'user' ? (
                <div className="whitespace-pre-wrap leading-relaxed">{msg.content}</div>
              ) : (
                <>
                  {/* AI message with loading state */}
                  {msg.isStreaming && !msg.content ? (
                    <div className="flex items-center gap-2 py-1">
                      <Loader2 size={14} className="animate-spin text-accent" />
                      <span className="text-sm text-text-muted">正在思考...</span>
                    </div>
                  ) : (
                    <div>
                      <MarkdownWithReferences
                        content={msg.isStreaming ? displayContent : msg.content}
                        references={msg.references}
                      />
                      {msg.isStreaming && (
                        <span className="inline-block w-0.5 h-3 bg-accent ml-0.5 animate-pulse" />
                      )}
                    </div>
                  )}

                  <div className="flex items-center gap-2 mt-2 pt-2 border-t border-border">
                    <button
                      onClick={() => handleSaveAsNote(msg.content)}
                      className="flex items-center gap-1 text-xs text-text-muted hover:text-accent transition-colors cursor-pointer"
                    >
                      <Save size={11} /> 保存为笔记
                    </button>
                  </div>
                </>
              )}
            </div>
            {msg.role === 'user' && (
              <div className="w-7 h-7 rounded-lg bg-accent/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                <span className="text-xs font-bold text-accent">我</span>
              </div>
            )}
          </motion.div>
        ))}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div className="px-4 pb-4 flex-shrink-0">
        <div className="chat-input-container relative rounded-2xl border border-border-light bg-bg-card transition-colors">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={isStreaming ? 'AI 正在生成中...' : '输入问题，或说"帮我生成思维导图"...'}
            rows={2}
            disabled={isStreaming}
            className={cn(
              'w-full bg-transparent text-sm px-4 py-3 resize-none outline-none',
              isStreaming
                ? 'text-text-muted placeholder:text-text-muted cursor-not-allowed'
                : 'text-text-primary placeholder:text-text-muted'
            )}
          />
          <div className="flex items-center justify-between px-3 pb-2">
            <div className="flex items-center gap-1.5">
              {selectedSources.length > 0 && (
                <span className="text-[10px] text-accent bg-accent-glow px-1.5 py-0.5 rounded">
                  基于 {selectedSources.length} 份资料
                </span>
              )}
            </div>
            {isStreaming ? (
              <button
                onClick={handleStop}
                className="p-2 rounded-lg bg-error/80 text-white hover:bg-error transition-all cursor-pointer shadow-md shadow-error/30"
              >
                <Square size={16} />
              </button>
            ) : (
              <button
                onClick={handleSend}
                disabled={!input.trim()}
                className={cn(
                  'p-2 rounded-lg transition-all cursor-pointer',
                  input.trim()
                    ? 'bg-accent text-white hover:bg-accent-light shadow-md shadow-accent/30'
                    : 'bg-bg-hover text-text-muted cursor-not-allowed'
                )}
              >
                <Send size={16} />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
