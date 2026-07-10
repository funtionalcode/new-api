import type { CliproxyAuthFileBinding } from '../../cliproxy-auth-files/types'

type XAIQuotaSnapshot = Pick<
  CliproxyAuthFileBinding,
  | 'last_usage_tokens'
  | 'last_usage_quota'
  | 'last_plan_type'
  | 'last_xai_weekly_percent'
  | 'last_xai_weekly_period_start_at'
  | 'last_xai_weekly_period_end_at'
  | 'last_xai_product_usage'
  | 'last_xai_on_demand_cap'
  | 'last_xai_on_demand_used'
  | 'last_xai_billing_period_end_at'
>

export type XAIQuotaProgress = {
  usedPercent: number
  remainingPercent: number
}

export type XAIProductUsage = XAIQuotaProgress & {
  label: string
  product: string
}

export type XAIMoneyUsage = {
  enabled: boolean
  remainingPercent: number
  remainingLabel: string
  limitLabel: string
}

export type XAIQuotaSummary = {
  planLabel: string
  weekly: XAIQuotaProgress & {
    available: boolean
    periodStartAt: number
    periodEndAt: number
  }
  products: XAIProductUsage[]
  payAsYouGo: XAIMoneyUsage
  monthly: XAIMoneyUsage & {
    billingPeriodEndAt: number
  }
}

type RawProductUsage = {
  product?: unknown
  usage_percent?: unknown
  usagePercent?: unknown
}

function normalizeCents(value: unknown): number {
  const amount = Number(value)
  if (!Number.isFinite(amount)) return 0
  return Math.max(0, Math.round(amount))
}

function normalizePercent(value: unknown): number {
  const percent = Number(value)
  if (!Number.isFinite(percent)) return 0
  return Math.min(100, Math.max(0, Math.round(percent)))
}

function formatUSDCents(value: number): string {
  return new Intl.NumberFormat('en-US', {
    currency: 'USD',
    maximumFractionDigits: 2,
    minimumFractionDigits: 2,
    style: 'currency',
  }).format(normalizeCents(value) / 100)
}

function parseProductUsage(raw: string): XAIProductUsage[] {
  if (!raw.trim()) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.flatMap((item) => {
      if (!item || typeof item !== 'object') return []
      const productItem = item as RawProductUsage
      const product = String(productItem.product || '').trim()
      if (!product) return []
      const usedPercent = normalizePercent(
        productItem.usage_percent ?? productItem.usagePercent
      )
      return [
        {
          product,
          label: product.toLowerCase() === 'api' ? 'API' : product,
          usedPercent,
          remainingPercent: 100 - usedPercent,
        },
      ]
    })
  } catch {
    return []
  }
}

export function buildXAIQuotaSummary(
  snapshot: XAIQuotaSnapshot
): XAIQuotaSummary {
  const usedCents = normalizeCents(snapshot.last_usage_tokens)
  const monthlyLimitCents = normalizeCents(snapshot.last_usage_quota)
  const includedUsedCents = Math.min(usedCents, monthlyLimitCents)
  const monthlyRemainingCents = Math.max(
    0,
    monthlyLimitCents - includedUsedCents
  )
  const monthlyRemainingPercent =
    monthlyLimitCents > 0
      ? Math.round((monthlyRemainingCents / monthlyLimitCents) * 100)
      : 0

  const onDemandCapCents = normalizeCents(snapshot.last_xai_on_demand_cap)
  const explicitOnDemandUsedCents = normalizeCents(
    snapshot.last_xai_on_demand_used
  )
  const onDemandUsedCents =
    explicitOnDemandUsedCents > 0
      ? explicitOnDemandUsedCents
      : Math.max(0, usedCents - monthlyLimitCents)
  const onDemandRemainingCents = Math.max(
    0,
    onDemandCapCents - onDemandUsedCents
  )
  const onDemandRemainingPercent =
    onDemandCapCents > 0
      ? Math.round((onDemandRemainingCents / onDemandCapCents) * 100)
      : 0

  const weeklyUsedPercent = normalizePercent(snapshot.last_xai_weekly_percent)
  const products = parseProductUsage(snapshot.last_xai_product_usage)
  const planType = snapshot.last_plan_type.trim()
  const planLabel =
    planType || (monthlyLimitCents === 150000 ? 'SuperGrok Heavy' : 'SuperGrok')

  return {
    planLabel,
    weekly: {
      available:
        snapshot.last_xai_weekly_period_start_at > 0 ||
        snapshot.last_xai_weekly_period_end_at > 0 ||
        products.length > 0,
      usedPercent: weeklyUsedPercent,
      remainingPercent: 100 - weeklyUsedPercent,
      periodStartAt: snapshot.last_xai_weekly_period_start_at,
      periodEndAt: snapshot.last_xai_weekly_period_end_at,
    },
    products,
    payAsYouGo: {
      enabled: onDemandCapCents > 0,
      remainingPercent: onDemandRemainingPercent,
      remainingLabel: formatUSDCents(onDemandRemainingCents),
      limitLabel: formatUSDCents(onDemandCapCents),
    },
    monthly: {
      enabled: monthlyLimitCents > 0,
      remainingPercent: monthlyRemainingPercent,
      remainingLabel: formatUSDCents(monthlyRemainingCents),
      limitLabel: formatUSDCents(monthlyLimitCents),
      billingPeriodEndAt: snapshot.last_xai_billing_period_end_at,
    },
  }
}

export function maskXAIAccountName(value: string): string {
  const match = value.match(/^(xai[-_])([^@]+)(@.+)$/i)
  if (!match) return value
  const [, prefix, localPart, suffix] = match
  if (localPart.length <= 2) {
    return `${prefix}${'*'.repeat(localPart.length)}${suffix}`
  }
  return `${prefix}${localPart[0]}${'*'.repeat(localPart.length - 2)}${localPart.at(-1)}${suffix}`
}

export function remainingProgressClass(percent: number): string {
  const remainingPercent = normalizePercent(percent)
  if (remainingPercent < 30) {
    return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  }
  if (remainingPercent < 70) {
    return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  }
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}
