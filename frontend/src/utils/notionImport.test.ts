import test from 'node:test';
import assert from 'node:assert/strict';

import { dedupePageIds, isTerminalNotionTaskStatus, oauthMessage } from './notionImport.ts';

test('dedupePageIds keeps first occurrence and removes blanks', () => {
  assert.deepEqual(dedupePageIds(['p1', '', 'p1', 'p2']), ['p1', 'p2']);
});

test('terminal notion task statuses stop polling', () => {
  assert.equal(isTerminalNotionTaskStatus('completed'), true);
  assert.equal(isTerminalNotionTaskStatus('running'), false);
  assert.equal(isTerminalNotionTaskStatus('cancelled'), true);
});

test('oauthMessage maps a cancelled authorization to an actionable message', () => {
  assert.match(oauthMessage('error', 'denied'), /取消/);
});
