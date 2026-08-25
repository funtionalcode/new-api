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
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { BindingUsageCell } from './index'
import type { CliproxyAuthFileBinding } from './types'

const labels = {
  fiveHour: 'Five-hour Limit',
  weekly: 'Weekly Limit',
  fable: 'Fable Limit',
  codexFiveHour: 'Codex Five-hour Limit',
  codexWeekly: 'Codex Weekly Limit',
  reset: 'Reset',
  quota: 'Quota',
}

const xaiBinding: CliproxyAuthFileBinding = {
  id: 1,
  user_id: 1,
  username: 'root',
  remark: '',
  auth_index: 'xai-auth',
  auth_name: 'xai-account.json',
  auth_file: 'xai-account.json',
  description: '',
  account_id: '',
  enabled: true,
  last_refreshed_at: 0,
  last_usage_tokens: 0,
  last_usage_quota: 0,
  last_plan_type: '',
  last_five_hour_percent: 0,
  last_five_hour_reset_at: 0,
  last_weekly_percent: 0,
  last_weekly_reset_at: 0,
  last_codex_five_hour_percent: 0,
  last_codex_five_hour_reset_at: 0,
  last_codex_weekly_percent: 0,
  last_codex_weekly_reset_at: 0,
  last_claude_fable_percent: 0,
  last_claude_fable_reset_at: 0,
  last_xai_weekly_percent: 0,
  last_xai_weekly_period_start_at: 0,
  last_xai_weekly_period_end_at: 0,
  last_xai_product_usage: '',
  last_xai_on_demand_cap: 0,
  last_xai_on_demand_used: 0,
  last_xai_billing_period_end_at: 0,
  last_error: '',
  created_at: 0,
  updated_at: 0,
}

describe('xAI binding usage plan', () => {
  test('does not invent SuperGrok when the upstream plan is unknown', () => {
    render(<BindingUsageCell binding={xaiBinding} labels={labels} />)

    expect(screen.queryByText('SuperGrok')).not.toBeInTheDocument()
  })

  test('shows the exact upstream plan name', () => {
    render(
      <BindingUsageCell
        binding={{ ...xaiBinding, last_plan_type: 'SuperGrok Plus' }}
        labels={labels}
      />
    )

    expect(screen.getByText('SuperGrok Plus')).toBeInTheDocument()
  })
})
