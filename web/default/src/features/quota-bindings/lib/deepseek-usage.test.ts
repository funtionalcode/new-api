import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildDeepSeekMoneyUsage } from './deepseek-usage'

describe('deepseek money usage helpers', () => {
  test('builds remaining balance progress from wallets and monthly costs', () => {
    const usage = buildDeepSeekMoneyUsage({
      normalWallets: '[{"currency":"CNY","balance":"106.4682842800000000"}]',
      bonusWallets: '[{"currency":"CNY","balance":"3.5317157200000000"}]',
      monthlyCosts: '[{"currency":"CNY","amount":"10.0000000000000000"}]',
      todayCosts: '[{"currency":"CNY","amount":"1.2500000000000000"}]',
      todayUsedTokens: 100_000,
    })

    assert.equal(usage.currency, 'CNY')
    assert.equal(usage.remainingAmount, 110)
    assert.equal(usage.monthlyCostAmount, 10)
    assert.equal(usage.todayCostLabel, 'CNY 1.25')
    assert.equal(usage.todayTokenLabel, '10万')
    assert.equal(usage.todayTokenDetail, '100,000 token')
    assert.equal(usage.monthlyCostLabel, 'CNY 10')
    assert.equal(usage.remainingPercent, 92)
  })

  test('handles missing today costs', () => {
    const usage = buildDeepSeekMoneyUsage({
      normalWallets: '[]',
      bonusWallets: '',
      monthlyCosts: '',
      todayCosts: '',
    })

    assert.equal(usage.remainingLabel, '-')
    assert.equal(usage.todayCostLabel, '-')
    assert.equal(usage.todayTokenLabel, '-')
    assert.equal(usage.remainingPercent, 0)
  })
})
