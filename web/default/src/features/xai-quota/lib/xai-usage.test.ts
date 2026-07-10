import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildXAIQuotaSummary,
  maskXAIAccountName,
  remainingProgressClass,
} from './xai-usage'

describe('xAI quota card helpers', () => {
  test('builds the reference weekly and monthly quota card data', () => {
    const summary = buildXAIQuotaSummary({
      last_usage_tokens: 1768,
      last_usage_quota: 15000,
      last_plan_type: 'SuperGrok',
      last_xai_weekly_percent: 45,
      last_xai_weekly_period_start_at: 1783599360,
      last_xai_weekly_period_end_at: 1784204160,
      last_xai_product_usage: '[{"product":"Api","usage_percent":45}]',
      last_xai_on_demand_cap: 0,
      last_xai_on_demand_used: 0,
      last_xai_billing_period_end_at: 1785542400,
    })

    assert.equal(summary.planLabel, 'SuperGrok')
    assert.equal(summary.weekly.usedPercent, 45)
    assert.equal(summary.weekly.remainingPercent, 55)
    assert.equal(summary.products[0]?.label, 'API')
    assert.equal(summary.products[0]?.usedPercent, 45)
    assert.equal(summary.monthly.remainingPercent, 88)
    assert.equal(summary.monthly.remainingLabel, '$132.32')
    assert.equal(summary.monthly.limitLabel, '$150.00')
    assert.equal(summary.payAsYouGo.enabled, false)
  })

  test('handles enabled pay-as-you-go and over-limit monthly usage', () => {
    const summary = buildXAIQuotaSummary({
      last_usage_tokens: 17000,
      last_usage_quota: 15000,
      last_plan_type: '',
      last_xai_weekly_percent: 100,
      last_xai_weekly_period_start_at: 0,
      last_xai_weekly_period_end_at: 0,
      last_xai_product_usage: 'not-json',
      last_xai_on_demand_cap: 5000,
      last_xai_on_demand_used: 500,
      last_xai_billing_period_end_at: 0,
    })

    assert.equal(summary.planLabel, 'SuperGrok')
    assert.deepEqual(summary.products, [])
    assert.equal(summary.monthly.remainingPercent, 0)
    assert.equal(summary.monthly.remainingLabel, '$0.00')
    assert.equal(summary.payAsYouGo.enabled, true)
    assert.equal(summary.payAsYouGo.remainingPercent, 90)
    assert.equal(summary.payAsYouGo.remainingLabel, '$45.00')
    assert.equal(summary.payAsYouGo.limitLabel, '$50.00')
  })

  test('masks the email local part while preserving the auth file shape', () => {
    assert.equal(
      maskXAIAccountName('xai-duboislee1988@gmail.com.json'),
      'xai-d***********8@gmail.com.json'
    )
    assert.equal(maskXAIAccountName('xai-a@b.com.json'), 'xai-*@b.com.json')
    assert.equal(maskXAIAccountName('custom.json'), 'custom.json')
  })

  test('maps remaining percentages to quota risk colors', () => {
    assert.match(remainingProgressClass(100), /emerald/)
    assert.match(remainingProgressClass(70), /emerald/)
    assert.match(remainingProgressClass(69), /amber/)
    assert.match(remainingProgressClass(30), /amber/)
    assert.match(remainingProgressClass(29), /rose/)
    assert.match(remainingProgressClass(0), /rose/)
  })
})
