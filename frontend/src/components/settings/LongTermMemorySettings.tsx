import { useEffect, useState } from 'react';
import { AlertCircle, Check, Loader2, Trash2 } from 'lucide-react';

import * as userMemoryApi from '../../api/userMemory';
import type { MemoryType, UserMemory } from '../../api/userMemory';
import Button from '../ui/Button';
import Input from '../ui/Input';
import { getErrorMessage } from '../../utils/error';

type MemorySlot = {
  type: MemoryType;
  label: string;
  description: string;
  placeholder: string;
};

const memorySlots: MemorySlot[] = [
  {
    type: 'language',
    label: '默认语言',
    description: '跨会话默认使用的回答语言；当前支持中文和 English。',
    placeholder: '',
  },
  {
    type: 'answer_length',
    label: '回答篇幅',
    description: '默认的简洁或展开程度。',
    placeholder: '例如：先给五点以内的简洁结论，需要时再展开。',
  },
  {
    type: 'answer_style',
    label: '回答方式',
    description: '回答的组织和表达顺序。',
    placeholder: '例如：先给结论，再给理由和可执行步骤。',
  },
  {
    type: 'output_format',
    label: '输出格式',
    description: '常用的结果呈现方式。',
    placeholder: '例如：涉及比较时优先使用 Markdown 表格。',
  },
  {
    type: 'generation_style',
    label: '生成风格',
    description: '用于 PPT、笔记、测验和脑图的通用偏好。',
    placeholder: '例如：PPT 保持正式、简洁，每页一个核心观点。',
  },
  {
    type: 'custom_instruction',
    label: '通用偏好',
    description: '一条跨会话复用的其他输出偏好。',
    placeholder: '例如：术语第一次出现时附一句通俗解释。',
  },
];

function emptyDrafts(): Record<MemoryType, string> {
  return memorySlots.reduce((drafts, slot) => {
    drafts[slot.type] = '';
    return drafts;
  }, {} as Record<MemoryType, string>);
}

function memoryByType(memories: UserMemory[]): Partial<Record<MemoryType, UserMemory>> {
  return memories.reduce((result, memory) => {
    result[memory.type] = memory;
    return result;
  }, {} as Partial<Record<MemoryType, UserMemory>>);
}

export default function LongTermMemorySettings() {
  const [memories, setMemories] = useState<Partial<Record<MemoryType, UserMemory>>>({});
  const [drafts, setDrafts] = useState<Record<MemoryType, string>>(emptyDrafts);
  const [loading, setLoading] = useState(true);
  const [savingType, setSavingType] = useState<MemoryType | null>(null);
  const [deletingType, setDeletingType] = useState<MemoryType | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const response = await userMemoryApi.listUserMemories();
        if (cancelled) return;
        if (response.code !== 0) {
          setError(response.message || '加载长期记忆失败');
          return;
        }
        const nextMemories = memoryByType(response.data);
        setMemories(nextMemories);
        const nextDrafts = emptyDrafts();
        memorySlots.forEach((slot) => {
          nextDrafts[slot.type] = nextMemories[slot.type]?.content || '';
        });
        setDrafts(nextDrafts);
      } catch (requestError) {
        if (!cancelled) {
          setError(getErrorMessage(requestError, '加载长期记忆失败'));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  const save = async (type: MemoryType) => {
    const content = drafts[type].trim();
    if (!content) {
      setError('请先填写偏好内容；如需移除，请使用清除按钮。');
      return;
    }
    setSavingType(type);
    setError(null);
    try {
      const response = await userMemoryApi.upsertUserMemory(type, content);
      if (response.code !== 0) {
        setError(response.message || '保存长期记忆失败');
        return;
      }
      setMemories((current) => ({ ...current, [type]: response.data }));
      setDrafts((current) => ({ ...current, [type]: response.data.content }));
    } catch (requestError) {
      setError(getErrorMessage(requestError, '保存长期记忆失败'));
    } finally {
      setSavingType(null);
    }
  };

  const clear = async (type: MemoryType) => {
    if (!memories[type] || !window.confirm('清除后，该偏好不会再用于后续对话和生成。确定继续吗？')) {
      return;
    }
    setDeletingType(type);
    setError(null);
    try {
      const response = await userMemoryApi.deleteUserMemory(type);
      if (response.code !== 0) {
        setError(response.message || '清除长期记忆失败');
        return;
      }
      setMemories((current) => {
        const next = { ...current };
        delete next[type];
        return next;
      });
      setDrafts((current) => ({ ...current, [type]: '' }));
    } catch (requestError) {
      setError(getErrorMessage(requestError, '清除长期记忆失败'));
    } finally {
      setDeletingType(null);
    }
  };

  return (
    <section className="space-y-4">
      <div className="p-4 rounded-xl bg-accent/5 border border-accent/20">
        <h2 className="text-sm font-semibold text-text-primary">长期记忆</h2>
        <p className="text-xs text-text-muted mt-1 leading-5">
          这里保存的是跨会话的输出偏好。当前请求有明确要求时优先，记忆不是资料事实来源。
          请不要保存密码、令牌、身份证号或其他秘密。
        </p>
      </div>

      {error && (
        <div className="p-3 rounded-xl bg-error/5 border border-error/20 flex items-start gap-2">
          <AlertCircle size={16} className="text-error mt-0.5 flex-shrink-0" />
          <p className="text-sm text-error">{error}</p>
        </div>
      )}

      {loading ? (
        <div className="text-center py-8 text-text-muted">加载中...</div>
      ) : (
        <div className="space-y-4">
          {memorySlots.map((slot) => {
            const saved = memories[slot.type];
            const busy = savingType === slot.type || deletingType === slot.type;
            return (
              <div key={slot.type} className="bg-bg-card rounded-xl border border-border-light p-5">
                <div className="flex items-start justify-between gap-4 mb-3">
                  <div>
                    <h3 className="text-sm font-semibold text-text-primary">{slot.label}</h3>
                    <p className="text-xs text-text-muted mt-1">{slot.description}</p>
                  </div>
                  {saved && (
                    <span className="inline-flex items-center gap-1 text-xs text-success">
                      <Check size={14} /> 已保存
                    </span>
                  )}
                </div>
                {slot.type === 'language' ? (
                  <select
                    aria-label={slot.label}
                    value={drafts.language}
                    disabled={busy}
                    onChange={(event) => setDrafts((current) => ({ ...current, language: event.target.value }))}
                    className="w-full px-3 py-2 rounded-lg border border-border-light bg-bg-primary text-sm text-text-primary focus:outline-none focus:border-accent disabled:opacity-60"
                  >
                    <option value="">请选择默认语言</option>
                    <option value="中文">中文</option>
                    <option value="English">English</option>
                  </select>
                ) : (
                  <Input
                    aria-label={slot.label}
                    placeholder={slot.placeholder}
                    value={drafts[slot.type]}
                    maxLength={160}
                    disabled={busy}
                    onChange={(event) => setDrafts((current) => ({ ...current, [slot.type]: event.target.value }))}
                  />
                )}
                <div className="mt-3 flex items-center justify-between gap-2">
                  <span className="text-xs text-text-muted">
                    {slot.type === 'language' ? '当前支持中文和 English' : '最多 160 个字符'}
                  </span>
                  <div className="flex gap-2">
                    {saved && (
                      <Button variant="ghost" size="sm" disabled={busy} onClick={() => void clear(slot.type)}>
                        {deletingType === slot.type ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
                        清除
                      </Button>
                    )}
                    <Button size="sm" disabled={busy} onClick={() => void save(slot.type)}>
                      {savingType === slot.type ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                      保存
                    </Button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
