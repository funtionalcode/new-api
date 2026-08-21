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

import { buildGLMQuotaUsageSummary } from '../glm-usage'

describe('glm quota usage summary', () => {
  test('keeps reset timestamps only in hover detail windows', () => {
    const summary = buildGLMQuotaUsageSummary({
      last_five_hour_used_tokens: 1200,
      five_hour_limit_tokens: 6000,
      last_five_hour_percent: 20,
      last_five_hour_reset_at: 1783065600,
      last_weekly_used_tokens: 3000,
      weekly_limit_tokens: 10000,
      last_weekly_percent: 30,
      last_weekly_reset_at: 1783389960,
      last_mcp_monthly_used: 2,
      last_mcp_monthly_limit: 1000,
      last_mcp_monthly_percent: 1,
      last_mcp_monthly_reset_at: 1785745560,
    })

    assert.deepEqual(
      summary.visibleWindows.map((window) => Object.keys(window).sort()),
      [
        ['key', 'kind', 'limit', 'percent', 'used'],
        ['key', 'kind', 'limit', 'percent', 'used'],
        ['key', 'kind', 'limit', 'percent', 'used'],
      ]
    )
    assert.deepEqual(
      summary.detailWindows.map((window) => window.resetAt),
      [1783065600, 1783389960, 1785745560]
    )
  })
})
