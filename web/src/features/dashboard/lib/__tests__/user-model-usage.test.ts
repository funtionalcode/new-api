import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { QuotaDataItem } from '../../types'
import { processUserModelUsageData } from '../charts'

describe('user model usage aggregation', () => {
  test('merges rows by user and model while preserving user remarks', () => {
    const rows: QuotaDataItem[] = [
      {
        username: 'alice',
        remark: '核心用户',
        model_name: 'gpt-4.1',
        quota: 100,
        token_used: 10,
        count: 1,
        created_at: 1782835200,
      },
      {
        username: 'alice',
        model_name: 'gpt-4.1',
        quota: 200,
        token_used: 15,
        count: 2,
        created_at: 1782838800,
      },
      {
        username: 'alice',
        model_name: 'claude-sonnet-4',
        quota: 50,
        token_used: 100,
        count: 3,
        created_at: 1782835200,
      },
      {
        username: 'bob',
        model_name: 'grok-4',
        quota: 300,
        token_used: 20,
        count: 1,
        created_at: 1782835200,
      },
    ]

    const result = processUserModelUsageData(
      rows,
      Number.POSITIVE_INFINITY,
      'tokens'
    )

    assert.deepEqual(
      result.map((row) => ({
        username: row.username,
        remark: row.remark,
        userLabel: row.userLabel,
        modelName: row.modelName,
        requestCount: row.requestCount,
        tokenUsed: row.tokenUsed,
        quota: row.quota,
      })),
      [
        {
          username: 'alice',
          remark: '核心用户',
          userLabel: 'alice\n核心用户',
          modelName: 'claude-sonnet-4',
          requestCount: 3,
          tokenUsed: 100,
          quota: 50,
        },
        {
          username: 'alice',
          remark: '核心用户',
          userLabel: 'alice\n核心用户',
          modelName: 'gpt-4.1',
          requestCount: 3,
          tokenUsed: 25,
          quota: 300,
        },
        {
          username: 'bob',
          remark: '',
          userLabel: 'bob',
          modelName: 'grok-4',
          requestCount: 1,
          tokenUsed: 20,
          quota: 300,
        },
      ]
    )
  })

  test('sorts and limits users by amount metric when selected', () => {
    const rows: QuotaDataItem[] = [
      {
        username: 'alice',
        model_name: 'gpt-4.1',
        quota: 100,
        token_used: 500,
        count: 1,
        created_at: 1782835200,
      },
      {
        username: 'bob',
        model_name: 'grok-4',
        quota: 300,
        token_used: 20,
        count: 1,
        created_at: 1782835200,
      },
    ]

    const result = processUserModelUsageData(rows, 1, 'amount')

    assert.deepEqual(result, [
      {
        username: 'bob',
        remark: '',
        userLabel: 'bob',
        modelName: 'grok-4',
        requestCount: 1,
        tokenUsed: 20,
        quota: 300,
      },
    ])
  })
})
