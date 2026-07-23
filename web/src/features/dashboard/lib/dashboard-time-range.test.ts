import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  applyDashboardTimeRangePreset,
  detectDashboardTimeRangeDays,
  formatDashboardTimeRangeLabel,
} from './dashboard-time-range'

describe('dashboard time range controls', () => {
  test('applies a selected range without dropping other token filters', () => {
    const filters = applyDashboardTimeRangePreset(
      { time_granularity: 'day', username: 'root' },
      7,
      new Date('2026-07-17T02:16:41+08:00')
    )

    assert.deepEqual(filters, {
      time_granularity: 'day',
      username: 'root',
      start_timestamp: new Date('2026-07-10T02:16:41+08:00'),
      end_timestamp: new Date('2026-07-17T02:16:41+08:00'),
    })
  })

  test('detects exact quick-range presets', () => {
    const end = new Date('2026-07-17T02:16:41+08:00')
    const start = new Date('2026-07-10T02:16:41+08:00')

    assert.equal(detectDashboardTimeRangeDays(start, end), 7)
  })

  test('leaves custom ranges unselected', () => {
    const end = new Date('2026-07-17T02:16:41+08:00')
    const start = new Date('2026-07-10T14:16:41+08:00')

    assert.equal(detectDashboardTimeRangeDays(start, end), null)
  })

  test('formats the visible range with seconds', () => {
    assert.equal(
      formatDashboardTimeRangeLabel(
        new Date(2026, 6, 16, 2, 16, 41),
        new Date(2026, 6, 17, 2, 16, 41)
      ),
      '2026-07-16 02:16:41 ~ 2026-07-17 02:16:41'
    )
  })
})
