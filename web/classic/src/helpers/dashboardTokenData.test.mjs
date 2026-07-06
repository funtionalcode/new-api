import test from 'node:test';
import assert from 'node:assert/strict';

import { processTokenData } from './dashboardTokenData.js';

test('processTokenData ranks token usage and aggregates overflow rows', () => {
  const result = processTokenData(
    [
      { token_id: 2, token_name: 'prod-key', token_used: 30, quota: 100, count: 1 },
      { token_id: 2, token_name: 'prod-key', token_used: 20, quota: 50, count: 2 },
      { token_id: 0, token_used: 7, quota: 10, count: 1 },
      { token_id: 3, token_name: 'backup-key', token_used: 5, quota: 4, count: 1 },
    ],
    { limit: 2, unknownLabel: '未知令牌', otherLabel: '其他' },
  );

  assert.deepEqual(result, [
    {
      Token: 'prod-key',
      TokenId: 2,
      Tokens: 50,
      Quota: 150,
      Count: 3,
    },
    {
      Token: '未知令牌',
      TokenId: 0,
      Tokens: 7,
      Quota: 10,
      Count: 1,
    },
    {
      Token: '其他',
      TokenId: null,
      Tokens: 5,
      Quota: 4,
      Count: 1,
    },
  ]);
});
