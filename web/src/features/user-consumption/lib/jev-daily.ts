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
import type {
  JevDailyUsageRow,
  JevUsageMetrics,
  JevUsageStatus,
} from '../types'

export function getJevDayRange(date: Date): {
  start_timestamp: number
  end_timestamp: number
} {
  const start = new Date(date)
  start.setHours(0, 0, 0, 0)
  const end = new Date(start)
  // 使用当地日历的下一日，兼容夏令时的 23/25 小时日期。
  end.setDate(end.getDate() + 1)
  return {
    start_timestamp: Math.floor(start.getTime() / 1000),
    end_timestamp: Math.floor(end.getTime() / 1000),
  }
}

export const jevStatuses: JevUsageStatus[] = [
  'used',
  'not_used',
  'not_applied',
  'unknown',
  'evaluation_only',
  'no_activity',
]

export const emptyJevMetrics: JevUsageMetrics = {
  request_count: 0,
  prompt_tokens: 0,
  completion_tokens: 0,
  quota: 0,
  use_time_seconds: 0,
  cached_decisions: 0,
  evaluation_requests: 0,
  evaluation_prompt_tokens: 0,
  evaluation_completion_tokens: 0,
  evaluation_quota: 0,
}

export function sumJevUsage(rows: JevDailyUsageRow[]): JevUsageMetrics {
  const total = { ...emptyJevMetrics }
  for (const row of rows) {
    for (const key of Object.keys(total) as (keyof JevUsageMetrics)[]) {
      total[key] += row[key]
    }
  }
  return total
}

export type JevAuthTotal = JevUsageMetrics & {
  key: string
  user_id: number
  username: string
  auth_index: string
  auth_name: string
  provider: string
  bound: boolean
  used_requests: number
  not_used_requests: number
  not_applied_requests: number
  unknown_requests: number
}

export function groupJevUsageByAuth(rows: JevDailyUsageRow[]): JevAuthTotal[] {
  const files = new Map<string, JevAuthTotal>()
  for (const row of rows) {
    const key = `${row.user_id}\0${row.auth_index}`
    let file = files.get(key)
    if (!file) {
      file = {
        ...emptyJevMetrics,
        key,
        user_id: row.user_id,
        username: row.username,
        auth_index: row.auth_index,
        auth_name: row.auth_name,
        provider: row.provider,
        bound: row.bound,
        used_requests: 0,
        not_used_requests: 0,
        not_applied_requests: 0,
        unknown_requests: 0,
      }
      files.set(key, file)
    }
    for (const metric of Object.keys(
      emptyJevMetrics
    ) as (keyof JevUsageMetrics)[]) {
      file[metric] += row[metric]
    }
    if (row.jev_status === 'used') {
      file.used_requests += row.request_count
    }
    if (row.jev_status === 'not_used') {
      file.not_used_requests += row.request_count
    }
    if (row.jev_status === 'not_applied') {
      file.not_applied_requests += row.request_count
    }
    if (row.jev_status === 'unknown') {
      file.unknown_requests += row.request_count
    }
  }
  return [...files.values()].sort(
    (a, b) =>
      b.quota + b.evaluation_quota - (a.quota + a.evaluation_quota) ||
      a.key.localeCompare(b.key)
  )
}
