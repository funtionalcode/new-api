import type { CursorQuotaBinding, CursorQuotaModelUsage } from '../types'

type CursorQuotaUsageSource = Pick<
  CursorQuotaBinding,
  | 'last_plan_api_percent'
  | 'last_plan_total_percent'
  | 'last_plan_used_cents'
  | 'last_plan_limit_cents'
  | 'last_plan_remaining_cents'
  | 'last_on_demand_used_cents'
  | 'last_on_demand_limit_cents'
  | 'last_on_demand_remaining_cents'
  | 'last_grok_bot_usage_percent'
  | 'last_grok_bot_reset_at'
  | 'last_grok_bot_usage_available'
  | 'last_total_input_tokens'
  | 'last_total_output_tokens'
  | 'last_total_cache_write_tokens'
  | 'last_total_cache_read_tokens'
  | 'last_model_usage'
>

function nonNegativeNumber(value: unknown): number {
  const number = Number(value)
  if (!Number.isFinite(number) || number <= 0) return 0
  return number
}

function percent(value: unknown): number {
  return Math.min(100, nonNegativeNumber(value))
}

function tokenTotal(values: number[]): number {
  let total = 0
  for (const value of values) {
    total += nonNegativeNumber(value)
    if (total >= Number.MAX_SAFE_INTEGER) return Number.MAX_SAFE_INTEGER
  }
  return total
}

function parseModelUsage(raw: string): CursorQuotaModelUsage[] {
  if (!raw.trim()) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    const models: CursorQuotaModelUsage[] = []
    for (const item of parsed) {
      if (!item || typeof item !== 'object') continue
      const record = item as Record<string, unknown>
      if (typeof record.model !== 'string' || !record.model.trim()) continue
      models.push({
        model: record.model.trim(),
        input_tokens: nonNegativeNumber(record.input_tokens),
        output_tokens: nonNegativeNumber(record.output_tokens),
        cache_write_tokens: nonNegativeNumber(record.cache_write_tokens),
        cache_read_tokens: nonNegativeNumber(record.cache_read_tokens),
        total_tokens: nonNegativeNumber(record.total_tokens),
        total_cents: nonNegativeNumber(record.total_cents),
        tier: Math.max(0, Math.trunc(nonNegativeNumber(record.tier))),
      })
    }
    return models
  } catch {
    return []
  }
}

export function buildCursorQuotaUsageSummary(source: CursorQuotaUsageSource) {
  const inputTokens = nonNegativeNumber(source.last_total_input_tokens)
  const outputTokens = nonNegativeNumber(source.last_total_output_tokens)
  const cacheWriteTokens = nonNegativeNumber(
    source.last_total_cache_write_tokens
  )
  const cacheReadTokens = nonNegativeNumber(source.last_total_cache_read_tokens)

  return {
    apiPercent: percent(source.last_plan_api_percent),
    totalPercent: percent(source.last_plan_total_percent),
    grokBotUsagePercent: percent(source.last_grok_bot_usage_percent),
    grokBotResetAt: nonNegativeNumber(source.last_grok_bot_reset_at),
    grokBotUsageAvailable: source.last_grok_bot_usage_available === true,
    planUsedDollars: nonNegativeNumber(source.last_plan_used_cents) / 100,
    planLimitDollars: nonNegativeNumber(source.last_plan_limit_cents) / 100,
    planRemainingDollars:
      nonNegativeNumber(source.last_plan_remaining_cents) / 100,
    onDemandUsedDollars:
      nonNegativeNumber(source.last_on_demand_used_cents) / 100,
    onDemandLimitDollars:
      nonNegativeNumber(source.last_on_demand_limit_cents) / 100,
    onDemandRemainingDollars:
      nonNegativeNumber(source.last_on_demand_remaining_cents) / 100,
    inputTokens,
    outputTokens,
    cacheWriteTokens,
    cacheReadTokens,
    totalTokens: tokenTotal([
      inputTokens,
      outputTokens,
      cacheWriteTokens,
      cacheReadTokens,
    ]),
    models: parseModelUsage(source.last_model_usage),
  }
}
