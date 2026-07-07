import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { processUserTokenRankChartData } from './user-token-rank'
import type { FlowQuotaDataItem } from '../types'

describe('user token rank chart data', () => {
  test('aggregates flow rows into token ranking spec', () => {
    const rows: FlowQuotaDataItem[] = [
      {
        token_id: 1,
        token_name: 'primary',
        token_used: 40,
        count: 2,
        quota: 100,
        model_name: 'gpt-test',
        use_group: 'default',
      },
      {
        token_id: 1,
        token_name: 'primary',
        token_used: 20,
        count: 1,
        quota: 50,
        model_name: 'gpt-test',
        use_group: 'default',
      },
      {
        token_id: 2,
        token_name: 'backup',
        token_used: 90,
        count: 3,
        quota: 200,
        model_name: 'gpt-test',
        use_group: 'default',
      },
    ]

    const spec = processUserTokenRankChartData(rows, (key) => key, 10)
    const values = spec.data[0].values

    assert.equal(spec.title.text, 'Token Consumption Ranking')
    assert.equal(spec.title.subtext, 'Total: 150')
    assert.deepEqual(
      values.map((item: Record<string, unknown>) => [
        item.Token,
        item.rawValue,
      ]),
      [
        ['backup', 90],
        ['primary', 60],
      ]
    )
  })
})
