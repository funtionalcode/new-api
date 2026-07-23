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
import { TIME_RANGE_PRESETS } from '@/features/dashboard/constants'
import type { DashboardFilters } from '@/features/dashboard/types'
import dayjs from '@/lib/dayjs'
import { getRollingDateRange } from '@/lib/time'

export function applyDashboardTimeRangePreset(
  filters: DashboardFilters,
  days: number,
  now: Date = new Date()
): DashboardFilters {
  const { start, end } = getRollingDateRange(days, now)
  return {
    ...filters,
    start_timestamp: start,
    end_timestamp: end,
  }
}

export function detectDashboardTimeRangeDays(
  startTime?: Date,
  endTime?: Date
): number | null {
  if (!startTime || !endTime) return null

  const durationMs = endTime.getTime() - startTime.getTime()
  const preset = TIME_RANGE_PRESETS.find(
    (item) => Math.abs(durationMs - item.days * 86_400_000) <= 1000
  )
  return preset?.days ?? null
}

export function formatDashboardTimeRangeLabel(
  startTime?: Date,
  endTime?: Date
): string {
  if (!startTime || !endTime) return '-'

  return `${dayjs(startTime).format('YYYY-MM-DD HH:mm:ss')} ~ ${dayjs(endTime).format('YYYY-MM-DD HH:mm:ss')}`
}
