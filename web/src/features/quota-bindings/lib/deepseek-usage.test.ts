import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildDeepSeekMoneyUsage } from './deepseek-usage'

describe('deepseek money usage helpers', () => {
  test('builds DeepSeek balance and usage values from all three responses', () => {
    const usage = buildDeepSeekMoneyUsage({
      normalWallets: '[{"currency":"CNY","balance":"106.4682842800000000"}]',
      bonusWallets: '[{"currency":"CNY","balance":"3.5317157200000000"}]',
      monthlyCosts: '[{"currency":"CNY","amount":"10.0000000000000000"}]',
      todayCosts: '[{"currency":"CNY","amount":"1.2500000000000000"}]',
      totalCosts: '[{"currency":"CNY","amount":"40.0000000000000000"}]',
      monthlyUsedTokens: 100_000,
      requestCount: 4_083,
    })

    assert.equal(usage.currency, 'CNY')
    assert.equal(usage.remainingAmount, 110)
    assert.equal(usage.monthlyCostAmount, 10)
    assert.equal(usage.totalCostAmount, 40)
    assert.equal(usage.totalAmount, 150)
    assert.equal(usage.requestCount, 4_083)
    assert.equal(usage.todayCostLabel, 'CNY 1.25')
    assert.equal(usage.monthlyTokenLabel, '10万')
    assert.equal(usage.monthlyTokenDetail, '100,000 token')
    assert.equal(usage.monthlyCostLabel, 'CNY 10')
    assert.equal(usage.totalCostLabel, 'CNY 40')
    assert.equal(usage.remainingPercent, 73)
  })

  test('handles missing today costs', () => {
    const usage = buildDeepSeekMoneyUsage({
      normalWallets: '[]',
      bonusWallets: '',
      monthlyCosts: '',
      todayCosts: '',
      totalCosts: '',
    })

    assert.equal(usage.remainingLabel, '-')
    assert.equal(usage.todayCostLabel, '-')
    assert.equal(usage.totalCostLabel, '-')
    assert.equal(usage.monthlyTokenLabel, '-')
    assert.equal(usage.remainingPercent, 0)
  })
})
