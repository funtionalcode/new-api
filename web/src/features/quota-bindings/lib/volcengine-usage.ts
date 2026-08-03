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

import type { VolcengineQuotaBinding } from '../types'

export type VolcengineQuotaWindowKey =
  | 'fiveHour'
  | 'daily'
  | 'weekly'
  | 'monthly'

export type VolcengineQuotaWindow = {
  key: VolcengineQuotaWindowKey
  quota: number
  used: number
  percent: number
  subscribeAt: number
  resetAt: number
}

export type VolcengineQuotaUsageSummary = {
  visibleWindows: VolcengineQuotaWindow[]
  detailWindows: VolcengineQuotaWindow[]
}

function normalizeNonNegative(value: number | undefined): number {
  const normalized = Number(value || 0)
  if (!Number.isFinite(normalized) || normalized < 0) return 0
  return normalized
}

function buildWindow(
  key: VolcengineQuotaWindowKey,
  quota: number,
  used: number,
  subscribeAt: number,
  resetAt: number
): VolcengineQuotaWindow {
  const normalizedQuota = normalizeNonNegative(quota)
  const normalizedUsed = normalizeNonNegative(used)
  const percent =
    normalizedQuota > 0
      ? Math.min(100, (normalizedUsed / normalizedQuota) * 100)
      : 0
  return {
    key,
    quota: normalizedQuota,
    used: normalizedUsed,
    percent,
    subscribeAt: normalizeNonNegative(subscribeAt),
    resetAt: normalizeNonNegative(resetAt),
  }
}

export function buildVolcengineQuotaUsageSummary(
  binding: VolcengineQuotaBinding
): VolcengineQuotaUsageSummary {
  const fiveHour = buildWindow(
    'fiveHour',
    binding.last_five_hour_quota,
    binding.last_five_hour_used_afp,
    binding.last_five_hour_subscribe_at,
    binding.last_five_hour_reset_at
  )
  const daily = buildWindow(
    'daily',
    binding.last_daily_quota,
    binding.last_daily_used_afp,
    binding.last_daily_subscribe_at,
    binding.last_daily_reset_at
  )
  const weekly = buildWindow(
    'weekly',
    binding.last_weekly_quota,
    binding.last_weekly_used_afp,
    binding.last_weekly_subscribe_at,
    binding.last_weekly_reset_at
  )
  const monthly = buildWindow(
    'monthly',
    binding.last_monthly_quota,
    binding.last_monthly_used_afp,
    binding.last_monthly_subscribe_at,
    binding.last_monthly_reset_at
  )

  return {
    visibleWindows: [fiveHour, weekly, monthly],
    detailWindows: [fiveHour, daily, weekly, monthly],
  }
}

export function formatVolcengineAFP(value: number): string {
  return Intl.NumberFormat(undefined, {
    maximumFractionDigits: 4,
  }).format(normalizeNonNegative(value))
}

export function formatVolcengineAFPPercent(value: number): string {
  const percent = Math.min(100, normalizeNonNegative(value))
  if (percent > 0 && percent < 0.1) return '<0.1%'
  return `${Intl.NumberFormat(undefined, {
    maximumFractionDigits: 2,
  }).format(percent)}%`
}
