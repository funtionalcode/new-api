import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildCliproxyUsageSummary,
  buildCliproxyXAIUsageSummary,
} from './usage-summary'

describe('cliproxy auth file usage summary', () => {
  test('keeps only primary usage windows in the table summary', () => {
    const summary = buildCliproxyUsageSummary({
      last_refreshed_at: 1,
      last_usage_tokens: 12345,
      last_usage_quota: 678,
      last_plan_type: 'pro',
      last_five_hour_percent: 24,
      last_five_hour_reset_at: 1783354083,
      last_weekly_percent: 85,
      last_weekly_reset_at: 1783388769,
      last_codex_five_hour_percent: 0,
      last_codex_five_hour_reset_at: 1783371908,
      last_codex_weekly_percent: 10,
      last_codex_weekly_reset_at: 1783394218,
      last_xai_on_demand_cap: 0,
      last_xai_billing_period_end_at: 0,
      last_error: '',
    })

    assert.equal(summary.hasUsageWindow, true)
    assert.deepEqual(
      summary.primaryWindows.map((item) => item.key),
      ['fiveHour', 'weekly']
    )
    assert.deepEqual(
      summary.detailWindows.map((item) => item.key),
      ['fiveHour', 'weekly', 'codexFiveHour', 'codexWeekly']
    )
  })

  test('falls back to legacy token usage when no window was refreshed', () => {
    const summary = buildCliproxyUsageSummary({
      last_refreshed_at: 0,
      last_usage_tokens: 2048,
      last_usage_quota: 100,
      last_plan_type: '',
      last_five_hour_percent: 0,
      last_five_hour_reset_at: 0,
      last_weekly_percent: 0,
      last_weekly_reset_at: 0,
      last_codex_five_hour_percent: 0,
      last_codex_five_hour_reset_at: 0,
      last_codex_weekly_percent: 0,
      last_codex_weekly_reset_at: 0,
      last_xai_on_demand_cap: 0,
      last_xai_billing_period_end_at: 0,
      last_error: '',
    })

    assert.equal(summary.hasUsageWindow, false)
    assert.equal(summary.primaryWindows.length, 0)
    assert.equal(summary.detailWindows.length, 0)
  })

  test('formats xAI billing values as USD cents with remaining percent', () => {
    const summary = buildCliproxyXAIUsageSummary({
      last_usage_tokens: 0,
      last_usage_quota: 15000,
      last_xai_weekly_percent: 0,
      last_xai_weekly_period_end_at: 0,
      last_xai_product_usage: '',
      last_xai_on_demand_cap: 0,
      last_xai_billing_period_end_at: 1785542400,
    })

    assert.equal(summary.remainingPercent, 100)
    assert.equal(summary.remainingLabel, '$150.00')
    assert.equal(summary.quotaLabel, '$150.00')
    assert.equal(summary.onDemandCapLabel, '$0.00')
    assert.equal(summary.billingPeriodEndAt, 1785542400)
  })

  test('builds xAI weekly, API, and monthly windows for the auth file table', () => {
    const summary = buildCliproxyXAIUsageSummary({
      last_usage_tokens: 1768,
      last_usage_quota: 15000,
      last_xai_weekly_percent: 45,
      last_xai_weekly_period_end_at: 1784204160,
      last_xai_product_usage: '[{"product":"Api","usage_percent":63}]',
      last_xai_on_demand_cap: 2500,
      last_xai_billing_period_end_at: 1785542400,
    })

    assert.deepEqual(summary.primaryWindows, [
      {
        key: 'weekly',
        percent: 45,
        resetAt: 1784204160,
      },
      {
        key: 'api',
        percent: 63,
        resetAt: 1784204160,
      },
      {
        key: 'monthly',
        percent: 12,
        resetAt: 1785542400,
      },
    ])
  })
})
