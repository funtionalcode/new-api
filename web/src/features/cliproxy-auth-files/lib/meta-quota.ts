export type MetaQuotaWindow = {
  usedPercent: number | null
  resetAt: number | null
  durationMinutes: number | null
}

export type MetaQuota = {
  plan?: string
  subscriptionActive?: boolean
  window: MetaQuotaWindow
  weekly: MetaQuotaWindow
}

export function parseMetaQuota(raw?: string): MetaQuota | null | undefined {
  if (!raw?.trim()) return undefined
  try {
    const value: unknown = JSON.parse(raw)
    if (!value || typeof value !== 'object' || Array.isArray(value)) return null
    const quota = value as Record<string, unknown>
    return {
      plan: typeof quota.plan === 'string' ? quota.plan : undefined,
      subscriptionActive:
        typeof quota.subscription_active === 'boolean'
          ? quota.subscription_active
          : undefined,
      window: parseMetaQuotaWindow(quota.window),
      weekly: parseMetaQuotaWindow(quota.weekly),
    }
  } catch {
    return null
  }
}

function parseMetaQuotaWindow(value: unknown): MetaQuotaWindow {
  const window =
    value && typeof value === 'object' && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : {}
  const percent = window.used_percent
  const reset = window.reset_at
  const duration = window.duration_minutes
  return {
    usedPercent:
      typeof percent === 'number' && Number.isFinite(percent)
        ? Math.min(100, Math.max(0, percent))
        : null,
    resetAt:
      typeof reset === 'number' &&
      Number.isSafeInteger(reset) &&
      reset > 0 &&
      reset <= 253402300799
        ? reset
        : null,
    durationMinutes:
      typeof duration === 'number' && Number.isFinite(duration) && duration > 0
        ? duration
        : null,
  }
}
