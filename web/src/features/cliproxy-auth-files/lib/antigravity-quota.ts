export type AntigravityBucketID =
  | 'gemini-5h'
  | 'gemini-weekly'
  | 'gemini-shared'
  | '3p-5h'
  | '3p-weekly'

export interface AntigravityQuotaBucket {
  bucket_id: AntigravityBucketID
  remaining_fraction: number
  reset_at: number
  disabled?: boolean
}

export function parseAntigravityQuota(
  raw?: string
): AntigravityQuotaBucket[] | null {
  if (!raw) return []
  try {
    const value: unknown = JSON.parse(raw)
    if (!Array.isArray(value)) return null
    const ids = new Set<string>()
    const valid = value.every((bucket): bucket is AntigravityQuotaBucket => {
      if (!bucket || typeof bucket !== 'object') return false
      if (ids.has(bucket.bucket_id)) return false
      ids.add(bucket.bucket_id)
      return (
        [
          'gemini-5h',
          'gemini-weekly',
          'gemini-shared',
          '3p-5h',
          '3p-weekly',
        ].includes(bucket.bucket_id) &&
        typeof bucket.reset_at === 'number' &&
        Number.isFinite(bucket.reset_at) &&
        typeof bucket.remaining_fraction === 'number' &&
        Number.isFinite(bucket.remaining_fraction) &&
        bucket.remaining_fraction >= 0 &&
        bucket.remaining_fraction <= 1 &&
        (bucket.disabled === undefined || bucket.disabled === false)
      )
    })
    if (!valid || value.length === 0) {
      return null
    }
    return value
  } catch {
    return null
  }
}
