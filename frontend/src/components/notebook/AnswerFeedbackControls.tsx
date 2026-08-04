import { useState, useRef, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ThumbsUp, ThumbsDown, X, Loader2 } from 'lucide-react';
import { cn } from '../../utils/cn';
import {
  upsertFeedback,
  deleteFeedback,
  UP_REASONS,
  DOWN_REASONS,
  type Rating,
  type ReasonCode,
} from '../../api/answerFeedback';
import type { ChatMessageFeedback } from '../../types';

interface AnswerFeedbackControlsProps {
  messageId: number;
  feedback?: ChatMessageFeedback | null;
  onFeedbackChange: (feedback: ChatMessageFeedback | null) => void;
}

export default function AnswerFeedbackControls({
  messageId,
  feedback,
  onFeedbackChange,
}: AnswerFeedbackControlsProps) {
  const [showPanel, setShowPanel] = useState(false);
  const [selectedRating, setSelectedRating] = useState<Rating | null>(null);
  const [selectedReason, setSelectedReason] = useState<ReasonCode | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        setShowPanel(false);
        setError(null);
      }
    }
    if (showPanel) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showPanel]);

  const handleOpenPanel = (rating: Rating) => {
    setSelectedRating(rating);
    setSelectedReason(feedback && feedback.rating === rating ? (feedback.reason as ReasonCode) : null);
    setError(null);
    setShowPanel(true);
  };

  const handleSubmit = async () => {
    if (!selectedRating || !selectedReason) return;
    setSubmitting(true);
    setError(null);
    try {
      const res = await upsertFeedback(messageId, selectedRating, selectedReason);
      if (res.code === 0 && res.data) {
        onFeedbackChange({
          rating: res.data.rating,
          reason: res.data.reason,
          updatedAt: res.data.updated_at,
        });
        setShowPanel(false);
      } else {
        setError(res.message || '提交失败，请重试');
      }
    } catch {
      setError('网络错误，请重试');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const res = await deleteFeedback(messageId);
      if (res.code === 0) {
        onFeedbackChange(null);
        setShowPanel(false);
      } else {
        setError(res.message || '撤销失败，请重试');
      }
    } catch {
      setError('网络错误，请重试');
    } finally {
      setSubmitting(false);
    }
  };

  const currentReasons = selectedRating === 'up' ? UP_REASONS : DOWN_REASONS;
  const hasFeedback = feedback !== null && feedback !== undefined;

  return (
    <div className="relative flex items-center gap-0.5">
      {/* Thumbs up */}
      <button
        onClick={() => handleOpenPanel('up')}
        disabled={submitting}
        aria-label="点赞此回答"
        className={cn(
          'group relative flex items-center justify-center w-7 h-7 rounded-lg transition-all duration-200 cursor-pointer',
          hasFeedback && feedback.rating === 'up'
            ? 'bg-accent/15 text-accent'
            : 'text-text-muted hover:text-accent hover:bg-accent/8'
        )}
      >
        <ThumbsUp size={13} strokeWidth={hasFeedback && feedback.rating === 'up' ? 2.5 : 2} />
      </button>

      {/* Thumbs down */}
      <button
        onClick={() => handleOpenPanel('down')}
        disabled={submitting}
        aria-label="点踩此回答"
        className={cn(
          'group relative flex items-center justify-center w-7 h-7 rounded-lg transition-all duration-200 cursor-pointer',
          hasFeedback && feedback.rating === 'down'
            ? 'bg-error/15 text-error'
            : 'text-text-muted hover:text-error hover:bg-error/8'
        )}
      >
        <ThumbsDown size={13} strokeWidth={hasFeedback && feedback.rating === 'down' ? 2.5 : 2} />
      </button>

      {/* Status pill */}
      <AnimatePresence>
        {hasFeedback && !showPanel && (
          <motion.span
            initial={{ opacity: 0, x: -4 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -4 }}
            className="text-[10px] text-text-muted ml-1 select-none"
          >
            已记录
          </motion.span>
        )}
      </AnimatePresence>

      {/* Reason selection panel */}
      <AnimatePresence>
        {showPanel && selectedRating && (
          <motion.div
            ref={panelRef}
            initial={{ opacity: 0, y: 6, scale: 0.96 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 6, scale: 0.96 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 w-60 bg-bg-card border border-border-light rounded-2xl shadow-2xl shadow-black/30 z-50 overflow-hidden"
          >
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-2.5 border-b border-border/60">
              <div className="flex items-center gap-2">
                {selectedRating === 'up' ? (
                  <div className="w-5 h-5 rounded-full bg-accent/15 flex items-center justify-center">
                    <ThumbsUp size={10} className="text-accent" />
                  </div>
                ) : (
                  <div className="w-5 h-5 rounded-full bg-error/15 flex items-center justify-center">
                    <ThumbsDown size={10} className="text-error" />
                  </div>
                )}
                <span className="text-xs font-medium text-text-primary">
                  {selectedRating === 'up' ? '为什么觉得好' : '为什么觉得不好'}
                </span>
              </div>
              <button
                onClick={() => { setShowPanel(false); setError(null); }}
                className="w-5 h-5 rounded-md flex items-center justify-center text-text-muted hover:text-text-primary hover:bg-bg-hover transition-colors cursor-pointer"
              >
                <X size={12} />
              </button>
            </div>

            {/* Reasons */}
            <div className="p-2 space-y-0.5">
              {currentReasons.map((r) => (
                <button
                  key={r.code}
                  onClick={() => setSelectedReason(r.code)}
                  disabled={submitting}
                  className={cn(
                    'w-full text-left px-3 py-2 rounded-xl text-xs transition-all duration-150 cursor-pointer',
                    selectedReason === r.code
                      ? selectedRating === 'up'
                        ? 'bg-accent/12 text-accent font-medium'
                        : 'bg-error/12 text-error font-medium'
                      : 'text-text-secondary hover:bg-bg-hover'
                  )}
                >
                  {r.label}
                </button>
              ))}
            </div>

            {/* Error */}
            <AnimatePresence>
              {error && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="px-4 py-1.5 text-[11px] text-error bg-error/5"
                >
                  {error}
                </motion.div>
              )}
            </AnimatePresence>

            {/* Footer */}
            <div className="flex items-center justify-between px-3 py-2.5 border-t border-border/60">
              {hasFeedback && (
                <button
                  onClick={handleDelete}
                  disabled={submitting}
                  className="text-[11px] text-text-muted hover:text-error transition-colors cursor-pointer"
                >
                  撤销评价
                </button>
              )}
              <div className="flex-1" />
              <button
                onClick={handleSubmit}
                disabled={!selectedReason || submitting}
                className={cn(
                  'flex items-center gap-1.5 text-[11px] font-medium px-3.5 py-1.5 rounded-lg transition-all duration-200 cursor-pointer',
                  selectedReason && !submitting
                    ? selectedRating === 'up'
                      ? 'bg-accent text-white hover:bg-accent-light shadow-sm shadow-accent/20'
                      : 'bg-error text-white hover:bg-error/80 shadow-sm shadow-error/20'
                    : 'bg-bg-hover text-text-muted cursor-not-allowed'
                )}
              >
                {submitting ? (
                  <Loader2 size={11} className="animate-spin" />
                ) : (
                  '确认'
                )}
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
