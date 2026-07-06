import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { formatAverageUseTimeSeconds } from './stat-format'

describe('usage log stat formatting', () => {
  test('formats backend average use_time as seconds', () => {
    assert.equal(formatAverageUseTimeSeconds(17), '17.0s')
    assert.equal(formatAverageUseTimeSeconds(3), '3.0s')
    assert.equal(formatAverageUseTimeSeconds(70), '1m 10s')
  })

  test('hides empty average use_time values', () => {
    assert.equal(formatAverageUseTimeSeconds(0), '-')
    assert.equal(formatAverageUseTimeSeconds(undefined), '-')
    assert.equal(formatAverageUseTimeSeconds(Number.NaN), '-')
  })
})
