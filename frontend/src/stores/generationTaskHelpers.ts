import type { GenerationTask } from '../api/generation';
import type { Note, NoteType } from '../types';

const generationTypeLabels: Record<NoteType, string> = {
  mindmap: '思维导图',
  ppt: '演示文稿',
  quiz: '测验',
  note: '笔记',
};

const generatedTaskNotePrefix = 'note-generation-task-';

export function generatedTaskNoteId(taskId: string): string {
  return `${generatedTaskNotePrefix}${taskId}`;
}

export function taskIdFromGeneratedNoteId(noteId: string): string | null {
  if (!noteId.startsWith(generatedTaskNotePrefix)) return null;
  const taskId = noteId.slice(generatedTaskNotePrefix.length);
  return taskId || null;
}

function timestampToISOString(value?: number): string {
  if (!value || value < 0) return new Date().toISOString();
  return new Date(value * 1000).toISOString();
}

export function createNoteFromGenerationTask(task: GenerationTask): Note | null {
  if (!task.result?.content || !task.notebook_id) return null;
  const type = task.type as NoteType;
  const firstLine = task.result.content.split('\n')[0] || '';
  const autoTitle = firstLine.replace(/^#+\s*/, '').trim() || `新${generationTypeLabels[type]}`;
  const createdAt = timestampToISOString(task.created_at);
  const updatedAt = timestampToISOString(task.updated_at || task.created_at);

  return {
    id: generatedTaskNoteId(task.task_id),
    title: autoTitle.slice(0, 40),
    type,
    content: task.result.content,
    isSource: false,
    notebookId: String(task.notebook_id),
    createdAt,
    updatedAt,
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
