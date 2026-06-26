import type { QuizQuestion } from '../../types';

// 判题结果：reference 表示不判分，仅展示参考答案（用于简答题等主观题）
export type JudgeResult = 'correct' | 'wrong' | 'reference';

// 某道题的用户作答状态
export interface AnswerState {
  selected?: number | null; // 单选 / 判断题：选中选项索引
  multiSelected?: number[]; // 多选题：选中选项索引列表
  fillValue?: string; // 填空 / 简答题：用户输入文本
}

// 归一化文本：去除所有空白并转小写，用于宽松匹配
function normalize(str: string): string {
  return str.replace(/\s+/g, '').toLowerCase();
}

// 多选题：把 answer（用中/英文分号分隔的正确选项文本）映射为 options 中的索引集合
export function getCorrectOptionIndices(q: QuizQuestion): number[] {
  const correctTexts = (q.answer || '')
    .split(/[；;]/)
    .map((t) => t.trim())
    .filter(Boolean);
  const indices: number[] = [];
  for (const text of correctTexts) {
    const idx = q.options.findIndex((o) => o.trim() === text);
    if (idx >= 0) indices.push(idx);
  }
  return indices;
}

// 是否已提交作答
export function isAnswered(q: QuizQuestion, s: AnswerState): boolean {
  switch (q.type) {
    case 'multi_choice':
      return (s.multiSelected?.length ?? 0) > 0;
    case 'fill_blank':
    case 'short_answer':
      return (s.fillValue || '').trim() !== '';
    default:
      return s.selected !== undefined && s.selected !== null;
  }
}

// 判题；未作答返回 null
export function judgeAnswer(q: QuizQuestion, s: AnswerState): JudgeResult | null {
  switch (q.type) {
    case 'multi_choice': {
      const selected = s.multiSelected ?? [];
      if (selected.length === 0) return null;
      const correctSet = new Set(getCorrectOptionIndices(q));
      // answer 无法映射到任何选项（数据异常）时，不强行判错，降级为展示参考答案
      if (correctSet.size === 0) return 'reference';
      const selectedSet = new Set(selected);
      if (selectedSet.size !== correctSet.size) return 'wrong';
      for (const idx of correctSet) {
        if (!selectedSet.has(idx)) return 'wrong';
      }
      return 'correct';
    }
    case 'fill_blank': {
      const user = (s.fillValue || '').trim();
      if (!user) return null;
      const answer = (q.answer || '').trim();
      if (!answer) return 'reference';
      // answer 可能含多个可接受答案（分号 / 斜杠 / 竖线分隔），任一归一化后相等即正确
      const acceptable = answer
        .split(/[；;|/]/)
        .map((t) => normalize(t))
        .filter(Boolean);
      return acceptable.includes(normalize(user)) ? 'correct' : 'wrong';
    }
    case 'short_answer': {
      // 主观题不判对错，提交后展示参考答案
      if (!(s.fillValue || '').trim()) return null;
      return 'reference';
    }
    default: {
      // single_choice / true_false
      if (s.selected === undefined || s.selected === null) return null;
      return s.selected === q.correctIndex ? 'correct' : 'wrong';
    }
  }
}
