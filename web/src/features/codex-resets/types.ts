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
export type CodexResetEvent = {
  id: number
  tweet_id: string
  tweet_url: string
  text: string
  announced_at: number
}

export type CodexResetsStats = {
  total: number
  last_reset_at: number
  days_since_last: number
  avg_interval_days: number
  longest_wait_days: number
  shortest_wait_days: number
}

export type CodexResetsHeatmapPoint = {
  date: string
  count: number
  level: number
}

export type CodexResetsIntervalPoint = {
  from_tweet_id: string
  to_tweet_id: string
  from_at: number
  to_at: number
  date: string
  interval_days: number
}

export type CodexResetsSyncInfo = {
  last_sync_at: number
  last_success_at: number
  last_error_at: number
  last_error: string
  last_tweet_id: string
  last_announced_at: number
  total_events: number
  source: string
}

export type CodexResetsData = {
  events: CodexResetEvent[]
  stats: CodexResetsStats
  charts: {
    heatmap: CodexResetsHeatmapPoint[]
    intervals: CodexResetsIntervalPoint[]
  }
  sync: CodexResetsSyncInfo
}

export type CodexResetsApiResponse = {
  success: boolean
  message?: string
  data?: CodexResetsData
}

export type CodexResetsSyncResponse = {
  success: boolean
  message?: string
  data?: {
    fetched: number
    inserted: number
    updated: number
    announced: number
    total_events: number
    latest_tweet_id?: string
    latest_announced_at?: number
  }
}
