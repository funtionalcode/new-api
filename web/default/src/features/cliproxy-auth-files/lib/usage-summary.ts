import type { CliproxyAuthFileBinding } from '../types'

export type CliproxyUsageWindowKey =
  | 'fiveHour'
  | 'weekly'
  | 'codexFiveHour'
  | 'codexWeekly'

export type CliproxyUsageWindow = {
  key: CliproxyUsageWindowKey
  percent: number
  resetAt: number
}

export type CliproxyUsageSummaryInput = Pick<
  CliproxyAuthFileBinding,
  | 'last_refreshed_at'
  | 'last_usage_tokens'
  | 'last_usage_quota'
  | 'last_plan_type'
  | 'last_five_hour_percent'
  | 'last_five_hour_reset_at'
  | 'last_weekly_percent'
  | 'last_weekly_reset_at'
  | 'last_codex_five_hour_percent'
  | 'last_codex_five_hour_reset_at'
  | 'last_codex_weekly_percent'
  | 'last_codex_weekly_reset_at'
  | 'last_error'
>

export type CliproxyUsageSummary = {
  hasUsageWindow: boolean
  primaryWindows: CliproxyUsageWindow[]
  detailWindows: CliproxyUsageWindow[]
}

export function buildCliproxyUsageSummary(
  binding: CliproxyUsageSummaryInput
): CliproxyUsageSummary {
  const hasUsageWindow =
    binding.last_refreshed_at > 0 ||
    binding.last_five_hour_reset_at > 0 ||
    binding.last_weekly_reset_at > 0 ||
    binding.last_codex_five_hour_reset_at > 0 ||
    binding.last_codex_weekly_reset_at > 0

  if (!hasUsageWindow) {
    return {
      hasUsageWindow: false,
      primaryWindows: [],
      detailWindows: [],
    }
  }

  const primaryWindows: CliproxyUsageWindow[] = [
    {
      key: 'fiveHour',
      percent: binding.last_five_hour_percent,
      resetAt: binding.last_five_hour_reset_at,
    },
    {
      key: 'weekly',
      percent: binding.last_weekly_percent,
      resetAt: binding.last_weekly_reset_at,
    },
  ]

  return {
    hasUsageWindow: true,
    primaryWindows,
    detailWindows: [
      ...primaryWindows,
      {
        key: 'codexFiveHour',
        percent: binding.last_codex_five_hour_percent,
        resetAt: binding.last_codex_five_hour_reset_at,
      },
      {
        key: 'codexWeekly',
        percent: binding.last_codex_weekly_percent,
        resetAt: binding.last_codex_weekly_reset_at,
      },
    ],
  }
}
