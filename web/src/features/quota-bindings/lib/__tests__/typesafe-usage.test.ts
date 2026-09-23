import { describe, expect, it } from 'vitest'

import type { TypeSafeUsageBucket } from '../../types'
import { summarizeTypeSafeUsage } from '../typesafe-usage'

const now = new Date(2026, 8, 23, 12).getTime()
const buckets: TypeSafeUsageBucket[] = [
  {
    day: new Date(2026, 8, 23, 8).toISOString(),
    apiKeyId: 'key-a',
    apiKeyName: 'A',
    userId: null,
    requests: 3,
    inputTokens: 1_000_000,
    outputTokens: 100,
  },
  {
    day: new Date(2026, 8, 23, 9).toISOString(),
    apiKeyId: null,
    apiKeyName: null,
    userId: 'user-a',
    requests: 2,
    inputTokens: 500_000,
    outputTokens: 50,
  },
  {
    day: new Date(2026, 8, 10, 8).toISOString(),
    apiKeyId: 'key-b',
    apiKeyName: 'B',
    userId: null,
    requests: 1,
    inputTokens: 100,
    outputTokens: 10,
  },
]

describe('TypeSafe console usage', () => {
  it('aggregates input/output and estimates cost without charging output tokens', () => {
    const summary = summarizeTypeSafeUsage(buckets, 7, 'all', false, now)
    expect(summary).toMatchObject({
      input: 1_500_000,
      output: 150,
      requests: 5,
      spend: 0.063,
    })
    expect(summary.points).toHaveLength(7)
    expect(summary.points.at(-1)).toMatchObject({
      input: 1_500_000,
      output: 150,
      requests: 5,
    })
  })
  it('filters API keys and playground independently', () => {
    expect(summarizeTypeSafeUsage(buckets, 7, 'api', true, now).requests).toBe(
      3
    )
    expect(
      summarizeTypeSafeUsage(buckets, 7, 'playground', true, now).requests
    ).toBe(2)
    expect(
      summarizeTypeSafeUsage(buckets, 30, 'key:b', true, now).requests
    ).toBe(0)
    expect(
      summarizeTypeSafeUsage(buckets, 30, 'key:key-b', true, now).requests
    ).toBe(1)
  })
  it('extends the time range and fills empty intervals without inventing usage', () => {
    const summary = summarizeTypeSafeUsage(buckets, 30, 'all', true, now)
    expect(summary.requests).toBe(6)
    expect(summary.points.filter((point) => point.requests > 0)).toHaveLength(3)
    expect(summarizeTypeSafeUsage([], 7, 'all', true, now)).toMatchObject({
      points: [],
      input: 0,
      output: 0,
      requests: 0,
      spend: 0,
    })
  })
})
