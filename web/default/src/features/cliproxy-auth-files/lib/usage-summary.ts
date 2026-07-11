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
  | 'last_xai_on_demand_cap'
  | 'last_xai_billing_period_end_at'
  | 'last_error'
>

export type CliproxyUsageSummary = {
  hasUsageWindow: boolean
  primaryWindows: CliproxyUsageWindow[]
  detailWindows: CliproxyUsageWindow[]
}

export type CliproxyXAIUsageSummaryInput = Pick<
  CliproxyAuthFileBinding,
  | 'last_usage_tokens'
  | 'last_usage_quota'
  | 'last_xai_weekly_percent'
  | 'last_xai_weekly_period_end_at'
  | 'last_xai_product_usage'
  | 'last_xai_on_demand_cap'
  | 'last_xai_billing_period_end_at'
>

export type CliproxyXAIUsageWindow = {
  key: 'weekly' | 'api' | 'monthly'
  percent: number
  resetAt: number
}

export type CliproxyXAIUsageSummary = {
  usedCents: number
  quotaCents: number
  remainingCents: number
  remainingPercent: number
  usedLabel: string
  quotaLabel: string
  remainingLabel: string
  onDemandCapLabel: string
  billingPeriodEndAt: number
  primaryWindows: CliproxyXAIUsageWindow[]
}

function normalizeCents(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.round(value))
}

function formatUSDCents(value: number): string {
  return new Intl.NumberFormat('en-US', {
    currency: 'USD',
    maximumFractionDigits: 2,
    minimumFractionDigits: 2,
    style: 'currency',
  }).format(normalizeCents(value) / 100)
}

function xaiAPIUsagePercent(rawProductUsage: string): number {
  try {
    const productUsage: unknown = JSON.parse(rawProductUsage)
    if (!Array.isArray(productUsage)) return 0

    for (const item of productUsage) {
      if (!item || typeof item !== 'object') continue
      const product = item as Record<string, unknown>
      if (
        String(product.product || '')
          .trim()
          .toLowerCase() !== 'api'
      ) {
        continue
      }

      const percent = Number(product.usage_percent ?? product.usagePercent)
      if (!Number.isFinite(percent)) return 0
      return Math.min(100, Math.max(0, Math.round(percent)))
    }
  } catch {
    return 0
  }
  return 0
}

export function buildCliproxyXAIUsageSummary(
  binding: CliproxyXAIUsageSummaryInput
): CliproxyXAIUsageSummary {
  const usedCents = normalizeCents(binding.last_usage_tokens)
  const quotaCents = normalizeCents(binding.last_usage_quota)
  const remainingCents = Math.max(0, quotaCents - usedCents)
  const remainingPercent =
    quotaCents > 0 ? Math.round((remainingCents / quotaCents) * 100) : 0
  const monthlyPercent =
    quotaCents > 0
      ? Math.min(100, Math.round((usedCents / quotaCents) * 100))
      : 0
  const apiUsagePercent = xaiAPIUsagePercent(binding.last_xai_product_usage)

  return {
    usedCents,
    quotaCents,
    remainingCents,
    remainingPercent,
    usedLabel: formatUSDCents(usedCents),
    quotaLabel: formatUSDCents(quotaCents),
    remainingLabel: formatUSDCents(remainingCents),
    onDemandCapLabel: formatUSDCents(binding.last_xai_on_demand_cap),
    billingPeriodEndAt: binding.last_xai_billing_period_end_at,
    primaryWindows: [
      {
        key: 'weekly',
        percent: binding.last_xai_weekly_percent,
        resetAt: binding.last_xai_weekly_period_end_at,
      },
      {
        key: 'api',
        percent: apiUsagePercent,
        resetAt: 0,
      },
      {
        key: 'monthly',
        percent: monthlyPercent,
        resetAt: binding.last_xai_billing_period_end_at,
      },
    ],
  }
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
