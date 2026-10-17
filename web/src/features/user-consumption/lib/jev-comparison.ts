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
import type { JevDailyUsageRow, JevUsageMetrics } from '../types'
import {
  emptyJevMetrics,
  groupJevUsageByAuth,
  type JevAuthTotal,
} from './jev-daily'

export interface JevComparisonMetrics extends JevUsageMetrics {
  used_requests: number
  evaluation_tokens: number
  combined_quota: number
  average_input_tokens: number | null
  average_output_tokens: number | null
  average_combined_quota: number | null
}

export function getJevComparisonMetrics(
  total: JevUsageMetrics,
  usedRequests: number
): JevComparisonMetrics {
  return {
    ...total,
    used_requests: usedRequests,
    evaluation_tokens:
      total.evaluation_prompt_tokens + total.evaluation_completion_tokens,
    combined_quota: total.quota + total.evaluation_quota,
    average_input_tokens:
      total.request_count > 0
        ? total.prompt_tokens / total.request_count
        : null,
    average_output_tokens:
      total.request_count > 0
        ? total.completion_tokens / total.request_count
        : null,
    average_combined_quota:
      total.request_count > 0
        ? (total.quota + total.evaluation_quota) / total.request_count
        : null,
  }
}

export function getJevMetricChange(
  baseline: number | null,
  current: number | null
): {
  difference: number | null
  percent: number | null
} {
  if (baseline === null || current === null) {
    return { difference: null, percent: null }
  }
  const difference = current - baseline
  let percent: number | null = null
  if (baseline !== 0) percent = (difference / Math.abs(baseline)) * 100
  else if (current === 0) percent = 0
  return { difference, percent }
}

export interface JevAuthComparison {
  key: string
  auth: JevAuthTotal
  baseline: JevComparisonMetrics
  current: JevComparisonMetrics
}

export function compareJevAuthUsage(
  baseline: JevDailyUsageRow[],
  current: JevDailyUsageRow[]
): JevAuthComparison[] {
  const rows = new Map<string, JevAuthComparison>()
  for (const [side, data] of [
    ['baseline', baseline],
    ['current', current],
  ] as const) {
    for (const auth of groupJevUsageByAuth(data)) {
      let row = rows.get(auth.key)
      if (!row) {
        row = {
          key: auth.key,
          auth,
          baseline: getJevComparisonMetrics(emptyJevMetrics, 0),
          current: getJevComparisonMetrics(emptyJevMetrics, 0),
        }
        rows.set(auth.key, row)
      }
      row.auth = auth
      row[side] = getJevComparisonMetrics(auth, auth.used_requests)
    }
  }
  return [...rows.values()].sort(
    (a, b) =>
      Math.abs(b.current.combined_quota - b.baseline.combined_quota) -
        Math.abs(a.current.combined_quota - a.baseline.combined_quota) ||
      a.key.localeCompare(b.key)
  )
}
