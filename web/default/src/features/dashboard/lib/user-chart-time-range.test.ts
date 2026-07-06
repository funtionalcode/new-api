import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildUserChartTimeRange,
  formatUserChartTimeRangeLabel,
} from './user-chart-time-range'
import type { UserChartsFilters } from '../types'

const baseFilters: UserChartsFilters = {
  timeGranularity: 'day',
  selectedRange: 7,
  topUserLimit: 10,
  metric: 'tokens',
}

describe('user chart time range helpers', () => {
  test('builds rolling range from selected preset days', () => {
    const range = buildUserChartTimeRange(
      baseFilters,
      new Date('2026-07-06T12:00:00+08:00')
    )

    assert.deepEqual(range, {
      start_timestamp: 1782705600,
      end_timestamp: 1783310400,
    })
  })

  test('uses custom dates when both boundaries are selected', () => {
    const range = buildUserChartTimeRange({
      ...baseFilters,
      customStartTime: new Date('2026-07-01T00:00:00+08:00'),
      customEndTime: new Date('2026-07-06T23:59:59+08:00'),
    })

    assert.deepEqual(range, {
      start_timestamp: 1782835200,
      end_timestamp: 1783353599,
    })
  })

  test('formats the visible selected range label', () => {
    const start = new Date(2026, 6, 1, 0, 0, 0)
    const end = new Date(2026, 6, 6, 23, 59, 59)
    assert.equal(
      formatUserChartTimeRangeLabel({
        start_timestamp: Math.floor(start.getTime() / 1000),
        end_timestamp: Math.floor(end.getTime() / 1000),
      }),
      '2026-07-01 00:00:00 ~ 2026-07-06 23:59:59'
    )
  })
})
