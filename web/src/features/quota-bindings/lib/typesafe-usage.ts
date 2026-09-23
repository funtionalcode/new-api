import type { TypeSafeUsageBucket } from '../types'

// 官方控制台的费用为估算值，价格来源：console.typesafe.ai/usage。
export const TYPESAFE_INPUT_USD_PER_MILLION = 0.042

export function summarizeTypeSafeUsage(
  buckets: TypeSafeUsageBucket[],
  days: number,
  traffic: string,
  hourly: boolean,
  now: number
) {
  const end = new Date(now)
  end.setHours(24, 0, 0, 0)
  const start = new Date(end)
  start.setDate(start.getDate() - days)
  const points = new Map<
    number,
    {
      time: number
      input: number
      output: number
      requests: number
      spend: number
    }
  >()
  let input = 0
  let output = 0
  let requests = 0
  for (const bucket of buckets) {
    const date = new Date(bucket.day)
    if (date < start || date >= end) continue
    if (traffic === 'api' && !bucket.apiKeyId) continue
    if (traffic === 'playground' && bucket.apiKeyId) continue
    if (traffic.startsWith('key:') && bucket.apiKeyId !== traffic.slice(4)) {
      continue
    }
    if (hourly) date.setMinutes(0, 0, 0)
    else date.setHours(0, 0, 0, 0)
    const time = date.getTime()
    const point = points.get(time) ?? {
      time,
      input: 0,
      output: 0,
      requests: 0,
      spend: 0,
    }
    point.input += bucket.inputTokens
    point.output += bucket.outputTokens
    point.requests += bucket.requests
    point.spend = (point.input * TYPESAFE_INPUT_USD_PER_MILLION) / 1_000_000
    points.set(time, point)
    input += bucket.inputTokens
    output += bucket.outputTokens
    requests += bucket.requests
  }
  if (points.size > 0) {
    const cursor = new Date(start)
    while (cursor < end) {
      const time = cursor.getTime()
      if (!points.has(time)) {
        points.set(time, { time, input: 0, output: 0, requests: 0, spend: 0 })
      }
      if (hourly) cursor.setTime(time + 3_600_000)
      else cursor.setDate(cursor.getDate() + 1)
    }
  }
  return {
    points: [...points.values()].sort((a, b) => a.time - b.time),
    input,
    output,
    requests,
    spend: (input * TYPESAFE_INPUT_USD_PER_MILLION) / 1_000_000,
  }
}
