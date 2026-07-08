import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildStatsApiParams } from './utils'

describe('usage log API params', () => {
  test('uses the selected time range as the average use time window', () => {
    const startTime = new Date('2026-07-08T00:00:00+08:00').getTime()
    const endTime = new Date('2026-07-08T10:57:00+08:00').getTime()

    const params = buildStatsApiParams({
      searchParams: { startTime, endTime },
      isAdmin: true,
    })

    assert.equal(params.start_timestamp, Math.floor(startTime / 1000))
    assert.equal(params.end_timestamp, Math.floor(endTime / 1000))
    assert.equal(params.avg_start_timestamp, params.start_timestamp)
    assert.equal(params.avg_end_timestamp, params.end_timestamp)
  })
})
