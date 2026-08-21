import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import type { UserConsumptionSummary } from '../types'
import { processTokenQuotaRankChartData } from './token-quota-rank'

describe('user consumption token quota rank chart data', () => {
  test('aggregates consumption rows into quota ranking spec', () => {
    const rows: UserConsumptionSummary[] = [
      {
        user_id: 1,
        username: 'root',
        remark: '',
        token_id: 1,
        token_name: 'primary',
        auth_index: 'primary',
        auth_name: '',
        request_count: 2,
        prompt_tokens: 10,
        completion_tokens: 30,
        total_tokens: 40,
        quota: 100,
        last_called_at: 100,
      },
      {
        user_id: 2,
        username: 'demo',
        remark: '',
        token_id: 1,
        token_name: 'primary',
        auth_index: 'primary',
        auth_name: '',
        request_count: 1,
        prompt_tokens: 5,
        completion_tokens: 15,
        total_tokens: 20,
        quota: 50,
        last_called_at: 120,
      },
      {
        user_id: 1,
        username: 'root',
        remark: '',
        token_id: 2,
        token_name: 'backup',
        auth_index: 'backup',
        auth_name: '',
        request_count: 3,
        prompt_tokens: 20,
        completion_tokens: 70,
        total_tokens: 90,
        quota: 200,
        last_called_at: 130,
      },
    ]

    const spec = processTokenQuotaRankChartData(rows, (key) => key, 10)
    const values = spec.data[0].values

    assert.equal(spec.title.text, 'Quota Consumption Ranking')
    assert.deepEqual(
      values.map((item: Record<string, unknown>) => [
        item.Token,
        item.rawValue,
        item.requests,
        item.userCount,
      ]),
      [
        ['backup', 200, 3, 1],
        ['primary', 150, 3, 2],
      ]
    )
  })
})
