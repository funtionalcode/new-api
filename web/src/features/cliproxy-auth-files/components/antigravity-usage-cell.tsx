import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { toIntlLocale } from '@/i18n/languages'
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
  const numberFormat = new Intl.NumberFormat(
    toIntlLocale(i18n.resolvedLanguage),
    {
      maximumFractionDigits: 2,
    }
  )
  const groups = [
    { prefix: 'gemini-', label: t('Gemini Models') },
    { prefix: '3p-', label: t('Claude and GPT models') },
  ].map((group) => ({
    ...group,
    windows: ['5h', 'weekly'].map((window) => {
      const bucket = buckets.find(
        (item) => item.bucket_id === group.prefix + window
      )
      const remaining = bucket?.remaining_fraction
      const available = !!bucket && !bucket.disabled && remaining !== undefined
      const percent = available ? remaining * 100 : 0
      return {
        key: window,
        label: window === '5h' ? t('5-Hour Window') : t('Weekly Window'),
        available,
        percent,
        value: available
          ? `${t('Remaining')} ${numberFormat.format(percent)}%`
          : t('Unavailable'),
        resetAt: bucket?.reset_at || 0,
      }
    }),
  }))

  return (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger
          aria-label='Antigravity'
          render={
            <div
              role='button'
              tabIndex={0}
              className='focus-visible:ring-ring max-w-[460px] cursor-help space-y-2 rounded-sm outline-none focus-visible:ring-2'
            />
          }
        >
          {buckets.length === 0 ? (
            <span className='text-muted-foreground text-sm'>
              {t('No quota data')}
            </span>
          ) : (
            <div className='grid gap-2 sm:grid-cols-2'>
              {groups[0].windows.map((window) => (
                <div key={window.key} className='min-w-[140px] space-y-1'>
                  <div className='flex items-center justify-between gap-3 text-xs'>
                    <span className='text-muted-foreground'>
                      {window.label}
                    </span>
                    <span className='font-mono font-medium tabular-nums'>
                      {window.value}
                    </span>
                  </div>
                  {window.available ? (
                    <Progress
                      aria-label={`${groups[0].label} ${window.label} ${t('Remaining')}`}
                      value={window.percent}
                      className={cn(
                        'h-1.5',
                        window.percent <= 10
                          ? '[&_[data-slot=progress-indicator]]:bg-rose-500'
                          : '[&_[data-slot=progress-indicator]]:bg-emerald-500'
                      )}
                    />
                  ) : null}
                </div>
              ))}
            </div>
          )}
          {props.binding.last_error ? (
            <Badge variant='destructive' className='h-5 px-1.5 text-[11px]'>
              {t('Error')}
            </Badge>
          ) : null}
        </TooltipTrigger>
        <TooltipContent
          role='tooltip'
          side='top'
          align='start'
          className='max-w-[min(34rem,calc(100vw-2rem))] p-3 whitespace-normal'
        >
          <div className='grid gap-3'>
            {buckets.length > 0 ? (
              groups.map((group) => (
                <section
                  key={group.prefix}
                  aria-label={group.label}
                  className='space-y-2'
                >
                  <div className='font-semibold'>{group.label}</div>
                  {group.windows.map((window) => (
                    <div
                      key={window.key}
                      className='grid grid-cols-[minmax(8rem,1fr)_auto] gap-x-4 gap-y-0.5'
                    >
                      <span>{window.label}</span>
                      <span className='font-mono font-semibold'>
                        {window.value}
                      </span>
                      <span className='text-background/70 col-span-2'>
                        {t('Reset')}:{' '}
                        {window.resetAt > 0
                          ? formatTimestampToDate(window.resetAt)
                          : '-'}
                      </span>
                    </div>
                  ))}
                </section>
              ))
            ) : (
              <span>{t('No quota data')}</span>
            )}
            {props.binding.last_error ? (
              <div className='border-background/15 text-background/80 border-t pt-2 break-words whitespace-pre-wrap'>
                {props.binding.last_error}
              </div>
            ) : null}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
