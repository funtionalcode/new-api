/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import type { UserConsumptionSummary } from '../types'
import { processUserQuotaRankChartData } from './user-quota-rank'

describe('user consumption user quota rank chart data', () => {
  test('aggregates consumption rows into user quota ranking spec', () => {
    const rows: UserConsumptionSummary[] = [
      {
        user_id: 1,
        username: 'root',
        remark: 'admin',
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
        user_id: 1,
        username: 'root',
        remark: 'admin',
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
    ]

    const spec = processUserQuotaRankChartData(rows, (key) => key, 10)
    const values = spec.data[0].values

    assert.equal(spec.title.text, 'User Quota Consumption Ranking')
    assert.deepEqual(
      values.map((item: Record<string, unknown>) => [
        item.User,
        item.rawValue,
        item.requests,
        item.tokenCount,
      ]),
      [
        ['root\nadmin', 300, 5, 2],
        ['demo', 50, 1, 1],
      ]
    )
  })
})
