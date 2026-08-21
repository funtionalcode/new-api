import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildCursorQuotaUsageSummary } from '../cursor-usage'

describe('Cursor quota usage summary', () => {
  test('parses model usage and derives stable totals from a refreshed binding', () => {
    const summary = buildCursorQuotaUsageSummary({
      last_plan_api_percent: 10.558,
      last_plan_total_percent: 2.1152,
      last_plan_used_cents: 5288,
      last_plan_limit_cents: 40000,
      last_plan_remaining_cents: 34712,
      last_grok_bot_usage_percent: 37.5,
      last_grok_bot_reset_at: 1_786_838_400,
      last_grok_bot_usage_available: true,
      last_on_demand_used_cents: 2500,
      last_on_demand_limit_cents: 15000,
      last_on_demand_remaining_cents: 12500,
      last_total_input_tokens: 300,
      last_total_output_tokens: 50,
      last_total_cache_write_tokens: 3,
      last_total_cache_read_tokens: 12,
      last_model_usage: JSON.stringify([
        {
          model: 'claude-fable-5-thinking-high',
          input_tokens: 100,
          output_tokens: 20,
          cache_write_tokens: 3,
          cache_read_tokens: 4,
          total_tokens: 127,
          total_cents: 12.5,
          tier: 1,
        },
      ]),
    })

    assert.equal(summary.apiPercent, 10.558)
    assert.equal(summary.totalPercent, 2.1152)
    assert.equal(summary.totalTokens, 365)
    assert.equal(summary.planUsedDollars, 52.88)
    assert.equal(summary.onDemandUsedDollars, 25)
    assert.equal(summary.grokBotUsagePercent, 37.5)
    assert.equal(summary.grokBotResetAt, 1_786_838_400)
    assert.equal(summary.grokBotUsageAvailable, true)
    assert.equal(summary.models.length, 1)
    assert.equal(summary.models[0]?.total_tokens, 127)
  })

  test('clamps invalid percentages and ignores malformed model usage', () => {
    const summary = buildCursorQuotaUsageSummary({
      last_plan_api_percent: 150,
      last_plan_total_percent: -5,
      last_plan_used_cents: -1,
      last_plan_limit_cents: 0,
      last_plan_remaining_cents: 0,
      last_grok_bot_usage_percent: 150,
      last_grok_bot_reset_at: -1,
      last_grok_bot_usage_available: false,
      last_on_demand_used_cents: 0,
      last_on_demand_limit_cents: 0,
      last_on_demand_remaining_cents: 0,
      last_total_input_tokens: -1,
      last_total_output_tokens: 0,
      last_total_cache_write_tokens: 0,
      last_total_cache_read_tokens: 0,
      last_model_usage: '{bad json',
    })

    assert.equal(summary.apiPercent, 100)
    assert.equal(summary.totalPercent, 0)
    assert.equal(summary.totalTokens, 0)
    assert.equal(summary.planUsedDollars, 0)
    assert.equal(summary.grokBotUsagePercent, 100)
    assert.equal(summary.grokBotResetAt, 0)
    assert.equal(summary.grokBotUsageAvailable, false)
    assert.deepEqual(summary.models, [])
  })
})
