import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { formatTokenDetails, formatTokens } from './format'

describe('token formatting', () => {
  test('uses Chinese token units for dashboard-sized values', () => {
    assert.equal(formatTokens(0), '0')
    assert.equal(formatTokens(10), '10')
    assert.equal(formatTokens(100), '百')
    assert.equal(formatTokens(1000), '千')
    assert.equal(formatTokens(10_000), '万')
    assert.equal(formatTokens(100_000), '10万')
    assert.equal(formatTokens(1_000_000), '百万')
    assert.equal(formatTokens(10_000_000), '千万')
    assert.equal(formatTokens(100_000_000), '亿')
    assert.equal(formatTokens(1_000_000_000), '10亿')
    assert.equal(formatTokens(10_000_000_000), '100亿')
  })

  test('keeps exact token details for tooltips', () => {
    assert.equal(formatTokenDetails(123_456_789), '123,456,789 token')
  })
})
