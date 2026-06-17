import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Check, X, RotateCcw, ChevronLeft, ChevronRight } from 'lucide-react';
import type { QuizQuestion } from '../../types';
import Button from '../ui/Button';

interface QuizCardProps {
  content: string;
}

export default function QuizCard({ content }: QuizCardProps) {
  let questions: QuizQuestion[] = [];
  try {
    const parsed = JSON.parse(content);
    questions = parsed.questions || [];
  } catch {
    return (
      <div className="flex items-center justify-center h-64 text-text-muted text-sm">
        无法解析测验内容
      </div>
    );
  }

  const [currentIndex, setCurrentIndex] = useState(0);
  const [selectedAnswers, setSelectedAnswers] = useState<Record<string, number | null>>({});
  const [showExplanation, setShowExplanation] = useState<Record<string, boolean>>({});

  const question = questions[currentIndex];
  if (!question) return null;

  const selectedAnswer = selectedAnswers[question.id];
  const isAnswered = selectedAnswer !== undefined && selectedAnswer !== null;
  const isCorrect = selectedAnswer === question.correctIndex;

  const handleSelect = (optionIndex: number) => {
    if (isAnswered) return;
    setSelectedAnswers((prev) => ({ ...prev, [question.id]: optionIndex }));
    setShowExplanation((prev) => ({ ...prev, [question.id]: true }));
  };

  const handleRetry = () => {
    setSelectedAnswers((prev) => ({ ...prev, [question.id]: null }));
    setShowExplanation((prev) => ({ ...prev, [question.id]: false }));
  };

  const handleRetryAll = () => {
    setSelectedAnswers({});
    setShowExplanation({});
    setCurrentIndex(0);
  };

  const answeredCount = Object.values(selectedAnswers).filter((v) => v !== null && v !== undefined).length;
  const correctCount = questions.filter((q) => selectedAnswers[q.id] === q.correctIndex).length;

  return (
    <div className="p-6">
      {/* Progress bar */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <span className="text-sm text-text-secondary">
            {currentIndex + 1} / {questions.length}
          </span>
          {answeredCount > 0 && (
            <span className="text-xs text-accent bg-accent-glow px-2 py-0.5 rounded-full">
              正确 {correctCount}/{answeredCount}
            </span>
          )}
        </div>
        <Button variant="ghost" size="sm" onClick={handleRetryAll}>
          <RotateCcw size={12} /> 重新作答
        </Button>
      </div>

      {/* Progress dots */}
      <div className="flex items-center gap-1.5 mb-6">
        {questions.map((q, i) => {
          const qAnswer = selectedAnswers[q.id];
          const qAnswered = qAnswer !== undefined && qAnswer !== null;
          return (
            <button
              key={q.id}
              onClick={() => setCurrentIndex(i)}
              className={`w-2 h-2 rounded-full transition-all cursor-pointer ${
                i === currentIndex
                  ? 'w-6 bg-accent'
                  : qAnswered
                    ? qAnswer === q.correctIndex
                      ? 'bg-success'
                      : 'bg-error'
                    : 'bg-border-light'
              }`}
            />
          );
        })}
      </div>

      {/* Question card */}
      <AnimatePresence mode="wait">
        <motion.div
          key={question.id}
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: -20 }}
          className="bg-bg-card rounded-xl border border-border-light p-6"
        >
          <h4 className="text-base font-medium text-text-primary mb-5 leading-relaxed">
            {question.question}
          </h4>

          <div className="space-y-2.5">
            {question.options.map((option, i) => {
              const isSelected = selectedAnswer === i;
              const isCorrectOption = i === question.correctIndex;
              const showResult = isAnswered;

              return (
                <motion.button
                  key={i}
                  whileHover={!isAnswered ? { scale: 1.01 } : undefined}
                  whileTap={!isAnswered ? { scale: 0.99 } : undefined}
                  onClick={() => handleSelect(i)}
                  disabled={isAnswered}
                  className={`w-full flex items-center gap-3 p-3.5 rounded-xl border text-left transition-all cursor-pointer ${
                    showResult
                      ? isCorrectOption
                        ? 'border-success bg-success/10 text-success'
                        : isSelected
                          ? 'border-error bg-error/10 text-error'
                          : 'border-border-light bg-bg-tertiary text-text-muted'
                      : isSelected
                        ? 'border-accent bg-accent/10 text-accent'
                        : 'border-border-light bg-bg-tertiary text-text-primary hover:border-accent/40 hover:bg-accent/5'
                  }`}
                >
                  <div
                    className={`w-6 h-6 rounded-full flex items-center justify-center flex-shrink-0 text-xs font-bold ${
                      showResult
                        ? isCorrectOption
                          ? 'bg-success text-white'
                          : isSelected
                            ? 'bg-error text-white'
                            : 'bg-bg-hover text-text-muted'
                        : isSelected
                          ? 'bg-accent text-white'
                          : 'bg-bg-hover text-text-muted'
                    }`}
                  >
                    {showResult && isCorrectOption ? (
                      <Check size={13} />
                    ) : showResult && isSelected && !isCorrectOption ? (
                      <X size={13} />
                    ) : (
                      String.fromCharCode(65 + i)
                    )}
                  </div>
                  <span className="text-sm">{option}</span>
                </motion.button>
              );
            })}
          </div>

          {/* Explanation */}
          <AnimatePresence>
            {showExplanation[question.id] && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                className="overflow-hidden"
              >
                <div className="mt-5 p-4 rounded-xl bg-bg-tertiary border border-border-light">
                  <div className="flex items-center gap-2 mb-2">
                    {isCorrect ? (
                      <span className="text-sm font-medium text-success">✓ 回答正确</span>
                    ) : (
                      <span className="text-sm font-medium text-error">✗ 回答错误</span>
                    )}
                  </div>
                  <p className="text-sm text-text-secondary leading-relaxed">
                    {question.explanation}
                  </p>
                  {!isCorrect && (
                    <Button variant="ghost" size="sm" onClick={handleRetry} className="mt-3">
                      <RotateCcw size={12} /> 重新作答
                    </Button>
                  )}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      </AnimatePresence>

      {/* Navigation */}
      <div className="flex items-center justify-between mt-4">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setCurrentIndex(Math.max(0, currentIndex - 1))}
          disabled={currentIndex === 0}
        >
          <ChevronLeft size={14} /> 上一题
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setCurrentIndex(Math.min(questions.length - 1, currentIndex + 1))}
          disabled={currentIndex === questions.length - 1}
        >
          下一题 <ChevronRight size={14} />
        </Button>
      </div>
    </div>
  );
}
