import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { materializeCompletedGenerationTask } from './generationTaskHelpers.ts';

test('materializes completed task result even when task was not submitted in this page session', () => {
  const createdTaskIds = new Set<string>();

  const note = materializeCompletedGenerationTask(
    {
      task_id: 'task-1',
      user_id: 42,
      notebook_id: 10,
      type: 'note',
      status: 'completed',
      result: {
        type: 'note',
        content: '# Generated Note\n\nBody',
      },
      created_at: 100,
      updated_at: 101,
    },
    createdTaskIds,
  );

  assert.equal(note?.notebookId, '10');
  assert.equal(note?.id, 'note-generation-task-task-1');
  assert.equal(note?.title, 'Generated Note');
  assert.equal(note?.content, '# Generated Note\n\nBody');
  assert.equal(note?.createdAt, '1970-01-01T00:01:40.000Z');
  assert.equal(note?.updatedAt, '1970-01-01T00:01:41.000Z');
  assert.equal(createdTaskIds.has('task-1'), true);
});

test('uses deterministic note identity for generated task results', () => {
  const task = {
    task_id: 'same-task',
    user_id: 42,
    notebook_id: 10,
    type: 'quiz',
    status: 'completed',
    result: {
      type: 'quiz',
      content: '{"questions":[]}',
    },
    created_at: 200,
    updated_at: 201,
  } as const;

  const first = materializeCompletedGenerationTask(task, new Set<string>());
  const second = materializeCompletedGenerationTask(task, new Set<string>());

  assert.equal(first?.id, second?.id);
  assert.equal(first?.createdAt, second?.createdAt);
  assert.equal(first?.updatedAt, second?.updatedAt);
});

test('generateNote restarts task polling after registering submitted task', () => {
  const currentDir = dirname(fileURLToPath(import.meta.url));
  const storeSource = readFileSync(join(currentDir, 'useNotebookStore.ts'), 'utf8');

  const registerIndex = storeSource.indexOf('pendingTaskNotebookMap.set(submittedTask.task_id, notebookId);');
  assert.notEqual(registerIndex, -1);

  const afterRegister = storeSource.slice(registerIndex, storeSource.indexOf('return;', registerIndex));
  assert.match(afterRegister, /get\(\)\.connectGenerationTasks\(notebookId\);/);
});
