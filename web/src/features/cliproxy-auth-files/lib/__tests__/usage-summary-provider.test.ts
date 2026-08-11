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
import { describe, test } from 'node:test'

import {
  buildCliproxyUsageSummary,
  type CliproxyUsageSummaryInput,
} from '../usage-summary'

const binding: CliproxyUsageSummaryInput = {
  last_refreshed_at: 1,
  last_usage_tokens: 0,
  last_usage_quota: 0,
  last_plan_type: 'claude_max_5x',
  last_five_hour_percent: 6,
  last_five_hour_reset_at: 1786438800,
  last_weekly_percent: 2,
  last_weekly_reset_at: 1786957200,
  last_codex_five_hour_percent: 91,
  last_codex_five_hour_reset_at: 1786439900,
  last_codex_weekly_percent: 82,
  last_codex_weekly_reset_at: 1786958300,
  last_claude_fable_percent: 72,
  last_claude_fable_reset_at: 1787043600,
  last_xai_on_demand_cap: 0,
  last_xai_billing_period_end_at: 0,
  last_error: '',
}

describe('Claude auth file usage summary', () => {
  test('shows Fable and never reuses Codex detail windows', () => {
    const summary = buildCliproxyUsageSummary(binding, 'claude')

    assert.deepEqual(
      summary.primaryWindows.map((item) => item.key),
      ['fiveHour', 'weekly', 'fable']
    )
    assert.deepEqual(
      summary.detailWindows.map((item) => item.key),
      ['fiveHour', 'weekly', 'fable']
    )
  })

  test('omits Fable when the Claude response has no Fable limit', () => {
    const summary = buildCliproxyUsageSummary(
      {
        ...binding,
        last_claude_fable_percent: 0,
        last_claude_fable_reset_at: 0,
      },
      'claude'
    )

    assert.deepEqual(
      summary.primaryWindows.map((item) => item.key),
      ['fiveHour', 'weekly']
    )
  })
})
