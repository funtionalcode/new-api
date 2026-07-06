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
import dayjs from '@/lib/dayjs'
import { getRollingDateRange } from '@/lib/time'

import type { UserChartsFilters } from '../types'

export type UserChartUnixTimeRange = {
  start_timestamp: number
  end_timestamp: number
}

function toUnixSeconds(date: Date): number {
  return Math.floor(date.getTime() / 1000)
}

function hasValidCustomRange(filters: UserChartsFilters): boolean {
  if (!filters.customStartTime || !filters.customEndTime) return false
  return filters.customEndTime.getTime() >= filters.customStartTime.getTime()
}

export function buildUserChartTimeRange(
  filters: UserChartsFilters,
  now: Date = new Date()
): UserChartUnixTimeRange {
  if (hasValidCustomRange(filters)) {
    return {
      start_timestamp: toUnixSeconds(filters.customStartTime!),
      end_timestamp: toUnixSeconds(filters.customEndTime!),
    }
  }

  const { start, end } = getRollingDateRange(filters.selectedRange, now)
  return {
    start_timestamp: toUnixSeconds(start),
    end_timestamp: toUnixSeconds(end),
  }
}

export function formatUserChartTimeRangeLabel(
  range: UserChartUnixTimeRange
): string {
  return `${dayjs(range.start_timestamp * 1000).format('YYYY-MM-DD HH:mm:ss')} ~ ${dayjs(range.end_timestamp * 1000).format('YYYY-MM-DD HH:mm:ss')}`
}
