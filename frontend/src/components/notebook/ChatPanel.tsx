import { useState, useRef, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Send, Plus, MessageSquare, Trash2, Save, Loader2,
  ChevronDown, Sparkles, Edit3, Square, X, Copy, Check,
  Bot, Settings
} from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { visit } from 'unist-util-visit';
import { useNotebookStore } from '../../stores/useNotebookStore';
import { cn } from '../../utils/cn';
import { listLLMConfigs } from '../../api/userConfig';
import type { UserLLMConfig } from '../../api/userConfig';
import type { NoteType, Reference } from '../../types';
import type { Root, Text } from 'mdast';

// Reference popover component
function ReferencePopover({ references, startIndex = 1 }: { references: Reference[]; startIndex?: number }) {
  const [openIndex, setOpenIndex] = useState<number | null>(null);
  const [popoverStyle, setPopoverStyle] = useState<React.CSSProperties>({});
  const popoverRef = useRef<HTMLDivElement>(null);
  const buttonRefs = useRef<(HTMLButtonElement | null)[]>([]);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setOpenIndex(null);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleToggle = (idx: number, e: React.MouseEvent) => {
    e.stopPropagation();
    if (openIndex === idx) {
      setOpenIndex(null);
      return;
    }

    // Calculate position
    const button = buttonRefs.current[idx];
    if (button) {
      const rect = button.getBoundingClientRect();
      const viewportHeight = window.innerHeight;
      const popoverHeight = 200; // approximate height

      // Determine if popover should go above or below
      const spaceAbove = rect.top;
      const spaceBelow = viewportHeight - rect.bottom;

      let top: number;
      const left = rect.left + rect.width / 2;

      if (spaceAbove > popoverHeight || spaceAbove > spaceBelow) {
        // Show above
        top = rect.top - 8;
        setPopoverStyle({
          position: 'fixed',
          top: `${top}px`,
          left: `${left}px`,
          transform: 'translate(-50%, -100%)',
          zIndex: 9999,
        });
      } else {
        // Show below
        top = rect.bottom + 8;
        setPopoverStyle({
          position: 'fixed',
          top: `${top}px`,
          left: `${left}px`,
          transform: 'translateX(-50%)',
          zIndex: 9999,
        });
      }
    }

    setOpenIndex(idx);
  };

  return (
    <sup className="inline-flex gap-0.5 text-xs relative">
      {references.map((ref, idx) => (
        <span key={idx}>
          <button
            ref={(el) => { buttonRefs.current[idx] = el; }}
            onClick={(e) => handleToggle(idx, e)}
            className="inline-flex items-center justify-center w-4 h-4 rounded text-[10px] font-bold bg-accent/20 text-accent hover:bg-accent/30 transition-colors cursor-pointer"
          >
            {startIndex + idx}
          </button>
          {openIndex === idx && (
            <div
              ref={popoverRef}
              style={popoverStyle}
              className="w-72 bg-bg-card border border-border-light rounded-xl shadow-xl overflow-hidden"
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
    </sup>
  );
}

// Remark plugin to extract reference markers [1], [2], etc.
function remarkExtractReferences(references: Reference[]) {
  return (tree: Root) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    visit(tree, 'text', ((node: Text, index: number | undefined, parent: any) => {
      if (!parent || index === undefined) return;

      const refRegex = /\[(\d+)\]/g;
      const parts: Array<Text | { type: string; data: Record<string, unknown> }> = [];
      let lastIndex = 0;
      let match;

      while ((match = refRegex.exec(node.value)) !== null) {
        const refIdx = parseInt(match[1]) - 1;

        // Add text before the reference
        if (match.index > lastIndex) {
          parts.push({
            type: 'text',
            value: node.value.slice(lastIndex, match.index),
          } as Text);
        }

        // Add reference node if valid
        if (refIdx >= 0 && refIdx < references.length) {
          parts.push({
            type: 'referenceMarker',
            data: {
              hName: 'sup',
              hProperties: {
                className: 'ref-marker',
                'data-ref-idx': refIdx,
              },
              hChildren: [{ type: 'text', value: match[0] }],
            },
          });
        } else {
          parts.push({ type: 'text', value: match[0] } as Text);
        }

        lastIndex = match.index + match[0].length;
      }

      // Add remaining text
      if (lastIndex < node.value.length) {
        parts.push({
          type: 'text',
          value: node.value.slice(lastIndex),
        } as Text);
      }

      if (parts.length > 0) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        parent.children.splice(index, 1, ...parts as any);
        return index + parts.length;
      }
    }) as any);
  };
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

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const remarkPlugins: any[] = references && references.length > 0
    ? [remarkGfm, [remarkExtractReferences, references]]
    : [remarkGfm];

  return (
    <div className="prose prose-sm prose-invert max-w-none select-text">
      <Markdown
        remarkPlugins={remarkPlugins}
        components={{
          sup: (props) => {
            const refIdx = (props as Record<string, unknown>)['data-ref-idx'] as string | undefined;
            if (refIdx !== undefined && references) {
              const idx = parseInt(refIdx);
              const ref = references[idx];
              if (ref) {
                return (
                  <ReferencePopover
                    references={[ref]}
                    startIndex={idx + 1}
                  />
                );
              }
            }
            return <sup {...props} />;
          },
        }}
      >
        {content}
      </Markdown>
    </div>
  );
}

// Copy button component with feedback
function CopyButton({
  content,
  onCopy,
  variant = 'dark'
}: {
  content: string;
  onCopy: (content: string) => Promise<boolean>;
  variant?: 'light' | 'dark';
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    const success = await onCopy(content);
    if (success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const baseClasses = variant === 'light'
    ? 'text-white/70 hover:text-white'
    : 'text-text-muted hover:text-accent';

  return (
    <button
      onClick={handleCopy}
      className={`flex items-center gap-1 text-xs transition-colors cursor-pointer ${baseClasses}`}
    >
      {copied ? (
        <>
          <Check size={11} /> 已复制
        </>
      ) : (
        <>
          <Copy size={11} /> 复制
        </>
      )}
    </button>
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
  const [showModelList, setShowModelList] = useState(false);
  const [editingConvId, setEditingConvId] = useState<string | null>(null);
  const [editConvTitle, setEditConvTitle] = useState('');
  const [llmConfigs, setLlmConfigs] = useState<UserLLMConfig[]>([]);
  const [selectedLlmConfigId, setSelectedLlmConfigId] = useState<number>(0);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const prevConvIdRef = useRef<string | null>(null);
  const modelListRef = useRef<HTMLDivElement>(null);

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

  // Load LLM configs on mount
  useEffect(() => {
    listLLMConfigs().then((res) => {
      if (res.code === 0 && res.data) {
        const enabledConfigs = res.data.filter((c) => c.enabled);
        setLlmConfigs(enabledConfigs);
        if (enabledConfigs.length > 0 && selectedLlmConfigId === 0) {
          setSelectedLlmConfigId(enabledConfigs[0].id);
        }
      }
    }).catch((err) => {
      console.error('Failed to load LLM configs:', err);
    });
  }, []);

  // Close model dropdown on outside click
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (modelListRef.current && !modelListRef.current.contains(e.target as Node)) {
        setShowModelList(false);
      }
    }
    if (showModelList) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showModelList]);

  const handleCopy = useCallback(async (content: string) => {
    try {
      await navigator.clipboard.writeText(content);
      return true;
    } catch (err) {
      console.error('Failed to copy:', err);
      return false;
    }
  }, []);

  if (!notebook || !currentNotebookId) return null;

  // 只有已入库且选中的资料才算资料来源
  const selectedSources = notebook.sources.filter((s) => s.selected && s.vectorized && s.status !== 'error');

  const handleSend = async () => {
    if (!input.trim() || isStreaming || !conversation?.id) return;
    if (llmConfigs.length === 0) return;

    const messageContent = input.trim();
    setInput('');

    const sourceIds = selectedSources.map((s) => Number(s.id));

    try {
      await sendMessage(currentNotebookId, conversation.id, messageContent, sourceIds, selectedLlmConfigId);
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
      id: `note-${crypto.randomUUID()}`,
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
            <p className="text-sm text-text-secondary max-w-xs whitespace-pre-line">
              你好！我是你的知识库助手，可以帮你阅读资料、回答问题、总结内容。
            </p>
            <p className="text-xs text-text-muted max-w-xs mt-2">
              💡 在左侧「资料来源」中选择文档，对话时我会基于所选资料为你精准解答
            </p>
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
            {msg.role === 'user' ? (
              <div className="flex flex-col items-end gap-1.5 max-w-[80%]">
                <div className="rounded-2xl rounded-br-md px-4 py-3 text-sm bg-accent text-white">
                  <div className="whitespace-pre-wrap leading-relaxed select-text">{msg.content}</div>
                </div>
                <CopyButton content={msg.content} onCopy={handleCopy} variant="dark" />
              </div>
            ) : (
              <div className="group relative max-w-[80%] rounded-2xl rounded-bl-md px-4 py-3 text-sm bg-bg-card border border-border-light">
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

                {!msg.isStreaming && (
                  <div className="flex items-center gap-2 mt-2 pt-2 border-t border-border">
                    <CopyButton content={msg.content} onCopy={handleCopy} variant="dark" />
                    <button
                      onClick={() => handleSaveAsNote(msg.content)}
                      className="flex items-center gap-1 text-xs text-text-muted hover:text-accent transition-colors cursor-pointer"
                    >
                      <Save size={11} /> 保存为笔记
                    </button>
                  </div>
                )}
              </div>
            )}
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
            placeholder={isStreaming ? 'AI 正在生成中...' : '请输入问题'}
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
            <div className="flex items-center gap-2">
              {llmConfigs.length > 0 ? (
                <div className="relative" ref={modelListRef}>
                  <button
                    onClick={() => !isStreaming && setShowModelList(!showModelList)}
                    disabled={isStreaming}
                    className={cn(
                      'flex items-center gap-1.5 text-xs px-2 py-1 rounded-lg border transition-all cursor-pointer',
                      showModelList
                        ? 'bg-accent/10 border-accent/30 text-accent'
                        : 'bg-bg-hover border-border-light text-text-secondary hover:border-accent/30 hover:text-accent'
                    )}
                  >
                    <Bot size={12} />
                    <span className="max-w-[120px] truncate">
                      {llmConfigs.find((c) => c.id === selectedLlmConfigId)?.name || '选择模型'}
                    </span>
                    <ChevronDown size={11} className={cn('transition-transform', showModelList && 'rotate-180')} />
                  </button>

                  <AnimatePresence>
                    {showModelList && (
                      <motion.div
                        initial={{ opacity: 0, y: 4 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: 4 }}
                        className="absolute left-0 bottom-full mb-1 w-56 bg-bg-card border border-border-light rounded-xl shadow-xl z-50 overflow-hidden"
                      >
                        <div className="p-1.5">
                          {llmConfigs.map((config) => (
                            <button
                              key={config.id}
                              onClick={() => {
                                setSelectedLlmConfigId(config.id);
                                setShowModelList(false);
                              }}
                              className={cn(
                                'w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-xs transition-all cursor-pointer',
                                config.id === selectedLlmConfigId
                                  ? 'bg-accent/10 text-accent'
                                  : 'text-text-secondary hover:bg-bg-hover'
                              )}
                            >
                              <div className={cn(
                                'w-6 h-6 rounded-md flex items-center justify-center flex-shrink-0',
                                config.id === selectedLlmConfigId ? 'bg-accent/20' : 'bg-bg-hover'
                              )}>
                                <Bot size={12} className={config.id === selectedLlmConfigId ? 'text-accent' : 'text-text-muted'} />
                              </div>
                              <div className="flex-1 text-left min-w-0">
                                <div className="font-medium truncate">{config.name || config.model}</div>
                                <div className="text-[10px] text-text-muted truncate">{config.provider} · {config.model}</div>
                              </div>
                              {config.id === selectedLlmConfigId && (
                                <Check size={12} className="text-accent flex-shrink-0" />
                              )}
                            </button>
                          ))}
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              ) : (
                <span className="flex items-center gap-1 text-[11px] text-warning">
                  <Settings size={11} />
                  请先配置 LLM
                </span>
              )}
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
                disabled={!input.trim() || llmConfigs.length === 0}
                className={cn(
                  'p-2 rounded-lg transition-all cursor-pointer',
                  input.trim() && llmConfigs.length > 0
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
