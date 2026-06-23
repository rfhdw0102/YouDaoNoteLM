import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Check, X, RotateCcw, ChevronLeft, ChevronRight } from 'lucide-react';
import type { QuizQuestion } from '../../types';
import Button from '../ui/Button';

interface QuizCardProps {
  content: string;
}

export default function QuizCard({ content }: QuizCardProps) {
  let rawQuestions: any[] = [];
  try {
    const parsed = JSON.parse(content);
    rawQuestions = parsed.questions || [];
  } catch {
    return (
      <div className="flex items-center justify-center h-64 text-text-muted text-sm">
        无法解析测验内容
      </div>
    );
  }

  // 适配后端数据格式：后端 answer 是字符串（正确选项文本），前端需要 correctIndex（数字索引）
  // 同时生成缺失的 id 字段
  const normalizeQuestion = (q: any, index: number): QuizQuestion => {
    const options: string[] = q.options || [];
    let correctIndex = q.correctIndex;

    // 根据题型确定 correctIndex
    if (q.type === 'true_false') {
      // 判断题：answer 是 "正确" 或 "错误"
      if (correctIndex === undefined && typeof q.answer === 'string') {
        correctIndex = q.answer === '错误' ? 1 : 0;
      }
    } else if (q.type === 'multi_choice') {
      // 多选题：correctIndex 不适用，存储为 -1 表示多选
      correctIndex = -1;
    } else if (q.type === 'fill_blank' || q.type === 'short_answer') {
      // 填空/简答题：没有选项
      correctIndex = -2;
    } else {
      // 单选题：原有逻辑
      if (correctIndex === undefined && typeof q.answer === 'string') {
        correctIndex = options.indexOf(q.answer);
      }
    }

    if (correctIndex === undefined || correctIndex < 0) {
      if (q.type === 'multi_choice') correctIndex = -1;
      else if (q.type === 'fill_blank' || q.type === 'short_answer') correctIndex = -2;
      else correctIndex = 0;
    }

    return {
      id: q.id || `quiz-q-${index}`,
      type: q.type || 'single_choice',
      question: q.question || '',
      options,
      correctIndex,
      answer: q.answer || '',
      explanation: q.explanation || '',
      difficulty: q.difficulty,
    };
  };

  const questions: QuizQuestion[] = rawQuestions.map(normalizeQuestion);

  const [currentIndex, setCurrentIndex] = useState(0);
  const [selectedAnswers, setSelectedAnswers] = useState<Record<string, number | null>>({});
  const [showExplanation, setShowExplanation] = useState<Record<string, boolean>>({});
  const [multiSelectAnswers, setMultiSelectAnswers] = useState<Record<string, number[]>>({});
  const [fillAnswers, setFillAnswers] = useState<Record<string, string>>({});

  const question = questions[currentIndex];

  const isMultiChoice = question?.type === 'multi_choice';
  const isFillBlank = question?.type === 'fill_blank';
  const isShortAnswer = question?.type === 'short_answer';
  const isTrueFalse = question?.type === 'true_false';

  const multiSelected = multiSelectAnswers[question?.id || ''] || [];
  const isMultiAnswered = isMultiChoice && multiSelected.length > 0;

  const fillValue = fillAnswers[question?.id || ''] || '';
  const isFillAnswered = (isFillBlank || isShortAnswer) && fillValue.trim() !== '';

  if (!question) return null;

  const selectedAnswer = selectedAnswers[question.id];
  const isAnswered = isMultiChoice ? isMultiAnswered : (isFillBlank || isShortAnswer) ? isFillAnswered : selectedAnswer !== undefined && selectedAnswer !== null;
  const isCorrect = question.type === 'multi_choice' || question.type === 'fill_blank' || question.type === 'short_answer'
    ? false
    : selectedAnswer === question.correctIndex;

  const handleSelect = (optionIndex: number) => {
    if (isMultiChoice) {
      setMultiSelectAnswers((prev) => {
        const current = prev[question.id] || [];
        if (current.includes(optionIndex)) {
          return { ...prev, [question.id]: current.filter(x => x !== optionIndex) };
        }
        return { ...prev, [question.id]: [...current, optionIndex] };
      });
      return;
    }
    if (isAnswered) return;
    setSelectedAnswers((prev) => ({ ...prev, [question.id]: optionIndex }));
    setShowExplanation((prev) => ({ ...prev, [question.id]: true }));
  };

  const handleRetry = () => {
    setSelectedAnswers((prev) => ({ ...prev, [question.id]: null }));
    setShowExplanation((prev) => ({ ...prev, [question.id]: false }));
    setMultiSelectAnswers((prev) => ({ ...prev, [question.id]: [] }));
    setFillAnswers((prev) => ({ ...prev, [question.id]: '' }));
  };

  const handleRetryAll = () => {
    setSelectedAnswers({});
    setShowExplanation({});
    setMultiSelectAnswers({});
    setFillAnswers({});
    setCurrentIndex(0);
  };

  const answeredCount = Object.values(selectedAnswers).filter((v) => v !== null && v !== undefined).length;
  const correctCount = questions.filter((q) => {
    if (q.type === 'multi_choice' || q.type === 'fill_blank' || q.type === 'short_answer') {
      return false;
    }
    return selectedAnswers[q.id] === q.correctIndex;
  }).length;

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
          const qSingleAnswer = selectedAnswers[q.id];
          const qMultiAnswered = multiSelectAnswers[q.id]?.length > 0;
          const qFillAnswered = (fillAnswers[q.id] || '').trim() !== '';
          const qAnswered = qSingleAnswer !== undefined && qSingleAnswer !== null
            || qMultiAnswered
            || qFillAnswered;
          const qAnswer = qSingleAnswer ?? (qMultiAnswered ? -1 : undefined);
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

          {/* Difficulty badge */}
          {question.difficulty && (
            <span className={`text-xs px-2 py-0.5 rounded-full mb-4 inline-block ${
              question.difficulty === 'easy' ? 'bg-success/10 text-success' :
              question.difficulty === 'medium' ? 'bg-accent/10 text-accent' :
              'bg-error/10 text-error'
            }`}>
              {question.difficulty === 'easy' ? '基础' : question.difficulty === 'medium' ? '中等' : '挑战'}
            </span>
          )}

          <div className="space-y-2.5">
            {/* 填空题 / 简答题 */}
            {(isFillBlank || isShortAnswer) && (
              <div className="space-y-3">
                {isFillBlank ? (
                  <input
                    type="text"
                    value={fillValue}
                    onChange={(e) => setFillAnswers(prev => ({ ...prev, [question.id]: e.target.value }))}
                    placeholder="请输入答案..."
                    disabled={isFillAnswered}
                    className="w-full p-3.5 rounded-xl border border-border-light bg-bg-tertiary text-text-primary text-sm focus:outline-none focus:border-accent"
                  />
                ) : (
                  <textarea
                    value={fillValue}
                    onChange={(e) => setFillAnswers(prev => ({ ...prev, [question.id]: e.target.value }))}
                    placeholder="请输入你的答案..."
                    disabled={isFillAnswered}
                    rows={3}
                    className="w-full p-3.5 rounded-xl border border-border-light bg-bg-tertiary text-text-primary text-sm focus:outline-none focus:border-accent resize-none"
                  />
                )}
                {!isFillAnswered && (
                  <Button variant="primary" size="sm" onClick={() => setShowExplanation(prev => ({ ...prev, [question.id]: true }))}>
                    提交答案
                  </Button>
                )}
              </div>
            )}

            {/* 选择题（单选/判断/多选） */}
            {!isFillBlank && !isShortAnswer && question.options.map((option, i) => {
              const isSelected = isMultiChoice ? multiSelected.includes(i) : selectedAnswer === i;
              const isCorrectOption = i === question.correctIndex;
              const showResult = isAnswered;

              return (
                <motion.button
                  key={i}
                  whileHover={!isAnswered && !isMultiChoice ? { scale: 1.01 } : undefined}
                  whileTap={!isAnswered && !isMultiChoice ? { scale: 0.99 } : undefined}
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
                    className={`w-6 h-6 ${isMultiChoice ? 'rounded-md' : 'rounded-full'} flex items-center justify-center flex-shrink-0 text-xs font-bold ${
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
                  {isMultiChoice && (
                    <div className="ml-auto">
                      <div className={`w-5 h-5 rounded border-2 flex items-center justify-center ${
                        isSelected ? 'border-accent bg-accent' : 'border-border-light'
                      }`}>
                        {isSelected && <Check size={12} className="text-white" />}
                      </div>
                    </div>
                  )}
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
