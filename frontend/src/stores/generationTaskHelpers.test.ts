import test from 'node:test';
import assert from 'node:assert/strict';

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
  assert.equal(note?.title, 'Generated Note');
  assert.equal(note?.content, '# Generated Note\n\nBody');
  assert.equal(createdTaskIds.has('task-1'), true);
});

