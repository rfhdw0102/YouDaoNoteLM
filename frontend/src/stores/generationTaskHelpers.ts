import type { GenerationTask } from '../api/generation';
import type { Note, NoteType } from '../types';

const generationTypeLabels: Record<NoteType, string> = {
  mindmap: '思维导图',
  ppt: '演示文稿',
  quiz: '测验',
  note: '笔记',
};

export function createNoteFromGenerationTask(task: GenerationTask): Note | null {
  if (!task.result?.content || !task.notebook_id) return null;
  const type = task.type as NoteType;
  const firstLine = task.result.content.split('\n')[0] || '';
  const autoTitle = firstLine.replace(/^#+\s*/, '').trim() || `新${generationTypeLabels[type]}`;

  return {
    id: `note-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    title: autoTitle.slice(0, 40),
    type,
    content: task.result.content,
    isSource: false,
    notebookId: String(task.notebook_id),
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
}

export function materializeCompletedGenerationTask(
  task: GenerationTask,
  createdTaskIds: Set<string>,
): Note | null {
  if (task.status !== 'completed' || createdTaskIds.has(task.task_id)) return null;
  const note = createNoteFromGenerationTask(task);
  if (!note) return null;
  createdTaskIds.add(task.task_id);
  return note;
}

