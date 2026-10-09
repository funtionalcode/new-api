export type AntigravityBucketID =
  | 'gemini-5h'
  | 'gemini-weekly'
  | '3p-5h'
  | '3p-weekly'

export interface AntigravityQuotaBucket {
  bucket_id: AntigravityBucketID
  remaining_fraction?: number
  reset_at: number
  disabled?: boolean
}

export function parseAntigravityQuota(raw?: string): AntigravityQuotaBucket[] {
  if (!raw) return []
  try {
    const value: unknown = JSON.parse(raw)
    if (!Array.isArray(value)) return []
    return value.filter((bucket): bucket is AntigravityQuotaBucket => {
      if (!bucket || typeof bucket !== 'object') return false
      return (
        ['gemini-5h', 'gemini-weekly', '3p-5h', '3p-weekly'].includes(
          bucket.bucket_id
        ) &&
        typeof bucket.reset_at === 'number' &&
        Number.isFinite(bucket.reset_at) &&
        (bucket.remaining_fraction === undefined ||
          (typeof bucket.remaining_fraction === 'number' &&
            Number.isFinite(bucket.remaining_fraction) &&
            bucket.remaining_fraction >= 0 &&
            bucket.remaining_fraction <= 1)) &&
        (bucket.disabled === undefined || typeof bucket.disabled === 'boolean')
      )
    })
  } catch {
    return []
  }
}
