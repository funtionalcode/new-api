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

export type QuotaProvider =
  | 'glm'
  | 'deepseek'
  | 'kimi'
  | 'volcengine'
  | 'cursor'

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
  last_five_hour_reset_at: number
  last_weekly_percent: number
  last_weekly_reset_at: number
  last_mcp_monthly_used: number
  last_mcp_monthly_limit: number
  last_mcp_monthly_percent: number
  last_mcp_monthly_reset_at: number
  last_model_call_count: number
  last_model_summary: string
}

export type DeepSeekQuotaBinding = QuotaBindingBase & {
  usage_amount_curl?: string
  usage_cost_curl?: string
  has_usage_amount_curl: boolean
  has_usage_cost_curl: boolean
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
  last_total_costs: string
  last_request_count: number
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

export type VolcengineQuotaBinding = QuotaBindingBase & {
  last_plan_type: string
  last_five_hour_quota: number
  last_five_hour_used_afp: number
  last_five_hour_subscribe_at: number
  last_five_hour_reset_at: number
  last_daily_quota: number
  last_daily_used_afp: number
  last_daily_subscribe_at: number
  last_daily_reset_at: number
  last_weekly_quota: number
  last_weekly_used_afp: number
  last_weekly_subscribe_at: number
  last_weekly_reset_at: number
  last_monthly_quota: number
  last_monthly_used_afp: number
  last_monthly_subscribe_at: number
  last_monthly_reset_at: number
}

export type CursorQuotaModelUsage = {
  model: string
  input_tokens: number
  output_tokens: number
  cache_write_tokens: number
  cache_read_tokens: number
  total_tokens: number
  total_cents: number
  tier: number
}

export type CursorQuotaBinding = QuotaBindingBase & {
  usage_amount_curl?: string
  usage_cost_curl?: string
  has_usage_amount_curl: boolean
  has_usage_cost_curl: boolean
  last_plan_name: string
  last_billing_cycle_start_at: number
  last_billing_cycle_end_at: number
  last_plan_used_cents: number
  last_plan_limit_cents: number
  last_plan_remaining_cents: number
  last_plan_api_percent: number
  last_plan_total_percent: number
  last_grok_bot_usage_percent: number
  last_grok_bot_reset_at: number
  last_grok_bot_usage_available: boolean
  last_on_demand_used_cents: number
  last_on_demand_limit_cents: number
  last_on_demand_remaining_cents: number
  last_total_input_tokens: number
  last_total_output_tokens: number
  last_total_cache_write_tokens: number
  last_total_cache_read_tokens: number
  last_total_cost_cents: number
  last_model_usage: string
}

export type QuotaBinding =
  | GLMQuotaBinding
  | DeepSeekQuotaBinding
  | KimiQuotaBinding
  | VolcengineQuotaBinding
  | CursorQuotaBinding

export type QuotaBindingFormData = {
  id?: number
  name: string
  note: string
  request_curl: string
  usage_amount_curl: string
  usage_cost_curl: string
  refresh_token: string
  proxy: string
  enabled: boolean
  plan_type: string
  five_hour_limit_tokens: number
  weekly_limit_tokens: number
}

export type QuotaBindingSavePayload = Omit<
  QuotaBindingFormData,
  | 'request_curl'
  | 'usage_amount_curl'
  | 'usage_cost_curl'
  | 'refresh_token'
  | 'proxy'
> & {
  request_curl?: string
  usage_amount_curl?: string
  usage_cost_curl?: string
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
