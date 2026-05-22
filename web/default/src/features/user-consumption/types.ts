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

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PageResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface UserConsumptionSummary {
  user_id: number
  username: string
  token_id: number
  token_name: string
  auth_index: string
  auth_name: string
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  quota: number
  last_called_at: number
}

export interface GetUserConsumptionParams {
  p?: number
  page_size?: number
  start_timestamp?: number
  end_timestamp?: number
  username?: string
  token_name?: string
  auth_index?: string
}

export type GetUserConsumptionResponse = ApiResponse<
  PageResponse<UserConsumptionSummary>
>
