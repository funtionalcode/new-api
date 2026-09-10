import { useTranslation } from 'react-i18next'

import { Progress } from '@/components/ui/progress'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { parseAntigravityQuota } from '../lib/antigravity-quota'
import type { CliproxyAuthFileBinding } from '../types'

export function AntigravityUsageCell(props: {
  binding: Pick<
    CliproxyAuthFileBinding,
    'last_antigravity_quota' | 'last_error'
  >
}) {
  const { t, i18n } = useTranslation()
  const buckets = parseAntigravityQuota(props.binding.last_antigravity_quota)
  const groups = [
    { prefix: 'gemini-', label: t('Gemini Models') },
    { prefix: '3p-', label: t('Claude and GPT models') },
  ]
  const numberFormat = new Intl.NumberFormat(i18n.resolvedLanguage, {
    maximumFractionDigits: 2,
  })

  return (
    <div className='max-w-[480px] min-w-[300px] space-y-2'>
      {buckets.length === 0 ? (
        <span className='text-muted-foreground text-sm'>
          {t('No quota data')}
        </span>
      ) : (
        <div className='grid gap-4 sm:grid-cols-2'>
          {groups.map((group) => (
            <section
              key={group.prefix}
              aria-label={group.label}
              className='min-w-0 space-y-2'
            >
              <div className='text-xs font-semibold'>{group.label}</div>
              {['5h', 'weekly'].map((window) => {
                const bucket = buckets.find(
                  (item) => item.bucket_id === group.prefix + window
                )
                const label =
                  window === '5h' ? t('Five-hour Limit') : t('Weekly Limit')
                const remaining = bucket?.remaining_fraction
                const available =
                  bucket && !bucket.disabled && remaining !== undefined
                const percent = available ? remaining * 100 : 0
                return (
                  <div key={window} className='space-y-1'>
                    <div className='flex items-center justify-between gap-3 text-xs'>
                      <span className='text-muted-foreground'>{label}</span>
                      <span className='font-mono tabular-nums'>
                        {available
                          ? `${t('Remaining')} ${numberFormat.format(percent)}%`
                          : t('Unavailable')}
                      </span>
                    </div>
                    {available ? (
                      <Progress
                        aria-label={`${group.label} ${label} ${t('Remaining')}`}
                        value={percent}
                        className={cn(
                          'h-1.5',
                          percent <= 10
                            ? '[&_[data-slot=progress-indicator]]:bg-rose-500'
                            : '[&_[data-slot=progress-indicator]]:bg-emerald-500'
                        )}
                      />
                    ) : null}
                    {bucket && bucket.reset_at > 0 ? (
                      <div className='text-muted-foreground text-[11px]'>
                        {t('Reset')}: {formatTimestampToDate(bucket.reset_at)}
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </section>
          ))}
        </div>
      )}
      {props.binding.last_error ? (
        <div
          role='alert'
          className='text-destructive max-w-[480px] text-xs break-words'
        >
          {props.binding.last_error}
        </div>
      ) : null}
    </div>
  )
}
