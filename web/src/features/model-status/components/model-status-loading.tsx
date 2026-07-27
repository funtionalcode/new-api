import { Skeleton } from '@/components/ui/skeleton'

const TODAY_SKELETON_KEYS = [
  'tokens',
  'quota',
  'requests',
  'rpm',
  'tpm',
] as const

const MODEL_SKELETON_KEYS = ['alpha', 'beta', 'gamma', 'delta'] as const

export function ModelStatusLoading() {
  return (
    <div className='space-y-8'>
      <Skeleton className='h-48 rounded-2xl' />
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'>
        {TODAY_SKELETON_KEYS.map((key) => (
          <Skeleton key={key} className='h-24 rounded-xl' />
        ))}
      </div>
      <div className='grid gap-4 lg:grid-cols-2'>
        {MODEL_SKELETON_KEYS.map((key) => (
          <Skeleton key={key} className='h-40 rounded-xl' />
        ))}
      </div>
    </div>
  )
}
