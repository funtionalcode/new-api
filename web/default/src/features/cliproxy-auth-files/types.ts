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

export interface CliproxyAuthFile {
  authIndex: string
  name: string
  authFile?: string
  accountId?: string
  planType?: string
  enabled: boolean
}

export interface CliproxyAuthFileBinding {
  id: number
  user_id: number
  username: string
  auth_index: string
  auth_name: string
  auth_file: string
  description: string
  account_id: string
  enabled: boolean
  last_refreshed_at: number
  last_usage_tokens: number
  last_usage_quota: number
  last_plan_type: string
  last_error: string
  created_at: number
  updated_at: number
}

export interface GetCliproxyAuthFileBindingsParams {
  p?: number
  page_size?: number
  username?: string
  auth_index?: string
  enabled?: string
}

export type GetCliproxyAuthFileBindingsResponse = ApiResponse<
  PageResponse<CliproxyAuthFileBinding>
>

export type GetCliproxyRemoteAuthFilesResponse = ApiResponse<CliproxyAuthFile[]>

export interface CliproxyAuthFileBindingFormData {
  user_id: number
  auth_index: string
  auth_name: string
  auth_file: string
  description: string
  account_id: string
  last_plan_type: string
  enabled: boolean
}
