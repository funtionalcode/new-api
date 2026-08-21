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

import type { VolcengineQuotaBinding } from '../../types'
import {
  buildVolcengineQuotaUsageSummary,
  formatVolcengineAFPPercent,
} from '../volcengine-usage'

function buildBinding(
  patch: Partial<VolcengineQuotaBinding> = {}
): VolcengineQuotaBinding {
  return {
    id: 1,
    name: 'Agent Plan',
    note: '',
    enabled: true,
    has_curl: true,
    last_refreshed_at: 0,
    last_error: '',
    created_at: 0,
    updated_at: 0,
    last_plan_type: 'medium',
    last_five_hour_quota: 10_000,
    last_five_hour_used_afp: 0.3736,
    last_five_hour_subscribe_at: 1_785_746_215,
    last_five_hour_reset_at: 1_785_764_215,
    last_daily_quota: 50_000,
    last_daily_used_afp: 0,
    last_daily_subscribe_at: 1_785_686_400,
    last_daily_reset_at: 1_785_772_800,
    last_weekly_quota: 35_000,
    last_weekly_used_afp: 0.3736,
    last_weekly_subscribe_at: 1_785_686_400,
    last_weekly_reset_at: 1_786_291_200,
    last_monthly_quota: 100_000,
    last_monthly_used_afp: 0.3736,
    last_monthly_subscribe_at: 1_785_745_776,
    last_monthly_reset_at: 1_788_451_199,
    ...patch,
  }
}

describe('VolcEngine AFP usage summary', () => {
  test('shows console windows and keeps the hidden daily window in details', () => {
    const summary = buildVolcengineQuotaUsageSummary(buildBinding())

    assert.deepEqual(
      summary.visibleWindows.map((window) => window.key),
      ['fiveHour', 'weekly', 'monthly']
    )
    assert.deepEqual(
      summary.detailWindows.map((window) => window.key),
      ['fiveHour', 'daily', 'weekly', 'monthly']
    )
    assert.equal(summary.visibleWindows[0].percent, 0.003736)
    assert.equal(
      formatVolcengineAFPPercent(summary.visibleWindows[0].percent),
      '<0.1%'
    )
  })

  test('clamps invalid and over-quota usage before rendering progress', () => {
    const summary = buildVolcengineQuotaUsageSummary(
      buildBinding({
        last_five_hour_used_afp: -1,
        last_weekly_quota: 10,
        last_weekly_used_afp: 20,
      })
    )

    assert.equal(summary.visibleWindows[0].used, 0)
    assert.equal(summary.visibleWindows[0].percent, 0)
    assert.equal(summary.visibleWindows[1].percent, 100)
  })
})
