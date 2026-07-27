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

import type { GLMQuotaBinding } from '../types'

export type GLMQuotaUsageWindowKey = 'fiveHour' | 'weekly' | 'mcpMonthly'

export type GLMQuotaUsageValueKind = 'tokens' | 'count'

export type GLMQuotaUsageBar = {
  key: GLMQuotaUsageWindowKey
  kind: GLMQuotaUsageValueKind
  used: number
  limit: number
  percent: number
}

export type GLMQuotaUsageDetailWindow = GLMQuotaUsageBar & {
  resetAt: number
}

export type GLMQuotaUsageSummaryInput = Pick<
  GLMQuotaBinding,
  | 'last_five_hour_used_tokens'
  | 'five_hour_limit_tokens'
  | 'last_five_hour_percent'
  | 'last_five_hour_reset_at'
  | 'last_weekly_used_tokens'
  | 'weekly_limit_tokens'
  | 'last_weekly_percent'
  | 'last_weekly_reset_at'
  | 'last_mcp_monthly_used'
  | 'last_mcp_monthly_limit'
  | 'last_mcp_monthly_percent'
  | 'last_mcp_monthly_reset_at'
>

export type GLMQuotaUsageSummary = {
  visibleWindows: GLMQuotaUsageBar[]
  detailWindows: GLMQuotaUsageDetailWindow[]
}

export function buildGLMQuotaUsageSummary(
  binding: GLMQuotaUsageSummaryInput
): GLMQuotaUsageSummary {
  const detailWindows: GLMQuotaUsageDetailWindow[] = [
    {
      key: 'fiveHour',
      kind: 'tokens',
      used: binding.last_five_hour_used_tokens,
      limit: binding.five_hour_limit_tokens,
      percent: binding.last_five_hour_percent,
      resetAt: binding.last_five_hour_reset_at,
    },
    {
      key: 'weekly',
      kind: 'tokens',
      used: binding.last_weekly_used_tokens,
      limit: binding.weekly_limit_tokens,
      percent: binding.last_weekly_percent,
      resetAt: binding.last_weekly_reset_at,
    },
    {
      key: 'mcpMonthly',
      kind: 'count',
      used: binding.last_mcp_monthly_used,
      limit: binding.last_mcp_monthly_limit,
      percent: binding.last_mcp_monthly_percent,
      resetAt: binding.last_mcp_monthly_reset_at,
    },
  ]

  return {
    visibleWindows: detailWindows.map((window) => ({
      key: window.key,
      kind: window.kind,
      used: window.used,
      limit: window.limit,
      percent: window.percent,
    })),
    detailWindows,
  }
}
