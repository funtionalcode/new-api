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

export type QuotaProvider = 'glm' | 'deepseek' | 'kimi'

export type QuotaBindingBase = {
  id: number
  name: string
  note: string
  request_curl?: string
  proxy?: string
  enabled: boolean
  has_curl: boolean
  last_refreshed_at: number
  last_error: string
  created_at: number
  updated_at: number
}

export type GLMQuotaBinding = QuotaBindingBase & {
  plan_type: string
  five_hour_limit_tokens: number
  weekly_limit_tokens: number
  last_five_hour_used_tokens: number
  last_weekly_used_tokens: number
  last_five_hour_percent: number
  last_weekly_percent: number
  last_model_call_count: number
  last_model_summary: string
}

export type DeepSeekQuotaBinding = QuotaBindingBase & {
  last_monthly_limit_tokens: number
  last_monthly_used_tokens: number
  last_monthly_remaining_tokens: number
  last_monthly_percent: number
  last_total_available_tokens: number
  last_today_used_tokens: number
  last_normal_wallets: string
  last_bonus_wallets: string
  last_monthly_costs: string
  last_today_costs: string
}

export type KimiQuotaBinding = QuotaBindingBase & {
  refresh_token?: string
  has_refresh_token?: boolean
  last_current_quota: number
  last_voucher_current_quota: number
  last_accumulated_quota: number
  last_voucher_accumulated_quota: number
  last_voucher_expired_quota: number
  last_recharge_bonus_percent: number
  last_used_quota: number
  last_remaining_quota: number
  last_total_quota: number
  last_remaining_percent: number
}

export type QuotaBinding =
  | GLMQuotaBinding
  | DeepSeekQuotaBinding
  | KimiQuotaBinding

export type QuotaBindingFormData = {
  id?: number
  name: string
  note: string
  request_curl: string
  refresh_token: string
  proxy: string
  enabled: boolean
  plan_type: string
  five_hour_limit_tokens: number
  weekly_limit_tokens: number
}

export type QuotaBindingSavePayload = Omit<
  QuotaBindingFormData,
  'request_curl' | 'refresh_token' | 'proxy'
> & {
  request_curl?: string
  refresh_token?: string
  proxy?: string
}

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

export type PageData<T> = {
  items: T[]
  total: number
  page: number
  page_size: number
}
