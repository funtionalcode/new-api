import test from 'node:test';
import assert from 'node:assert/strict';

import {
  getCliproxyAuthProvider,
  getCliproxyRefreshStatus,
} from './cliproxyAuthFiles.js';

test('cliproxy refresh status keeps the backend failure detail', () => {
  const status = getCliproxyRefreshStatus({
    last_error: ' upstream returned 401 Unauthorized ',
    last_refreshed_at: 1783300000,
  });

  assert.deepEqual(status, {
    failed: true,
    labelKey: '刷新失败',
    error: 'upstream returned 401 Unauthorized',
    refreshedAt: 1783300000,
  });
});

test('cliproxy refresh status treats blank errors as success', () => {
  const status = getCliproxyRefreshStatus({
    last_error: '   ',
    last_refreshed_at: 1783300123,
  });

  assert.deepEqual(status, {
    failed: false,
    labelKey: '刷新成功',
    error: '',
    refreshedAt: 1783300123,
  });
});

test('cliproxy auth provider prefers explicit provider fields', () => {
  assert.equal(
    getCliproxyAuthProvider({
      provider: 'claude',
      auth_index: 'codex-user@example.com-free.json',
    }),
    'claude',
  );
});

test('cliproxy auth provider falls back to auth identifiers', () => {
  assert.equal(
    getCliproxyAuthProvider({
      auth_index: 'codex-user@example.com-pro.json',
    }),
    'codex',
  );
  assert.equal(
    getCliproxyAuthProvider({
      auth_name: 'claude-gooddgege@gmail.com.json',
    }),
    'claude',
  );
});

test('cliproxy auth provider returns unknown for unsupported values', () => {
  assert.equal(
    getCliproxyAuthProvider({
      type: 'gemini',
      auth_index: 'other-user.json',
    }),
    'unknown',
  );
});
