import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { QuotaDataItem } from '../../types'
import {
  processUserModelUsageChartData,
  processUserModelUsageData,
} from '../charts'

type UserModelUsageChartSpec = {
  data: Array<{ values: Array<Record<string, unknown>> }>
  xField: string
  yField: string
  seriesField: string
  direction: string
  stack: boolean
  title: { subtext: string }
}

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

  test('builds horizontal stacked chart rows for selected users and models', () => {
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

    const result = processUserModelUsageChartData(
      rows,
      (key) => key,
      10,
      'tokens'
    )
    const spec =
      result.spec_user_model_usage as unknown as UserModelUsageChartSpec

    assert.equal(result.userCount, 2)
    assert.equal(spec.xField, 'rawValue')
    assert.equal(spec.yField, 'UserLabel')
    assert.equal(spec.seriesField, 'Model')
    assert.equal(spec.direction, 'horizontal')
    assert.equal(spec.stack, true)
    assert.equal(spec.title.subtext, 'Total: 130')
    assert.deepEqual(
      spec.data[0].values.map((row) => ({
        user: row.User,
        label: row.UserLabel,
        model: row.Model,
        value: row.rawValue,
        tokens: row.Tokens,
        quota: row.Quota,
        requests: row.Requests,
      })),
      [
        {
          user: 'alice',
          label: 'alice\n核心用户',
          model: 'claude-sonnet-4',
          value: 100,
          tokens: 100,
          quota: 50,
          requests: 3,
        },
        {
          user: 'alice',
          label: 'alice\n核心用户',
          model: 'gpt-4.1',
          value: 10,
          tokens: 10,
          quota: 100,
          requests: 1,
        },
        {
          user: 'bob',
          label: 'bob',
          model: 'grok-4',
          value: 20,
          tokens: 20,
          quota: 300,
          requests: 1,
        },
      ]
    )
  })

  test('uses quota as chart value when amount metric is selected', () => {
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

    const result = processUserModelUsageChartData(
      rows,
      (key) => key,
      1,
      'amount'
    )
    const spec =
      result.spec_user_model_usage as unknown as UserModelUsageChartSpec

    assert.equal(result.userCount, 1)
    assert.deepEqual(spec.data[0].values, [
      {
        User: 'bob',
        UserLabel: 'bob',
        Remark: '',
        Model: 'grok-4',
        Requests: 1,
        Tokens: 20,
        Quota: 300,
        rawValue: 300,
      },
    ])
  })
})
