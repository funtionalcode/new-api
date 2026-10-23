import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { getCliproxyPlanLabelConfig } from '../lib/plan-label'
import type { CliproxyAuthFileBinding } from '../types'

export function AntigravityUsageCell(props: {
  binding: Pick<
    CliproxyAuthFileBinding,
    'last_antigravity_quota' | 'last_error'
  > &
    Partial<
      Pick<CliproxyAuthFileBinding, 'last_plan_type' | 'last_refreshed_at'>
    >
}) {
  const { t, i18n } = useTranslation()
  const buckets = parseAntigravityQuota(props.binding.last_antigravity_quota)
  const error = props.binding.last_error.trim()
  if (
    error ||
    buckets === null ||
    (buckets.length === 0 && props.binding.last_refreshed_at)
  ) {
    return (
      <Alert variant='destructive' className='max-w-[460px]'>
        <AlertTitle>{t('Error')}</AlertTitle>
        <AlertDescription className='text-xs break-words whitespace-pre-wrap'>
          {error ||
            t(
              'Invalid or incomplete Antigravity quota data. Refresh to view the upstream error.'
            )}
        </AlertDescription>
      </Alert>
    )
  }
  const plan = getCliproxyPlanLabelConfig(
    'antigravity',
    props.binding.last_plan_type
  )
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
    windows: buckets
      .filter((bucket) => bucket.bucket_id.startsWith(group.prefix))
      .map((bucket) => {
        const percent = (1 - bucket.remaining_fraction) * 100
        let progressColor = '[&_[data-slot=progress-indicator]]:bg-emerald-500'
        if (percent >= 90) {
          progressColor = '[&_[data-slot=progress-indicator]]:bg-rose-500'
        } else if (percent >= 70) {
          progressColor = '[&_[data-slot=progress-indicator]]:bg-amber-500'
        }
        return {
          key: bucket.bucket_id,
          label: bucket.bucket_id.endsWith('-5h')
            ? t('5-Hour Window')
            : t('Weekly Window'),
          percent,
          progressColor,
          value: `${numberFormat.format(percent)}%`,
          resetAt: bucket.reset_at,
        }
      }),
  }))
  const primaryGroup = groups.find((group) => group.windows.length > 0)

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
              {primaryGroup?.windows.map((window) => (
                <div key={window.key} className='min-w-[140px] space-y-1'>
                  <div className='flex items-center justify-between gap-3 text-xs'>
                    <span className='text-muted-foreground'>
                      {window.label}
                    </span>
                    <span className='font-mono font-medium tabular-nums'>
                      {window.value}
                    </span>
                  </div>
                  <Progress
                    aria-label={`${primaryGroup.label} ${window.label} ${t('Used')}`}
                    value={window.percent}
                    className={cn('h-1.5', window.progressColor)}
                  />
                </div>
              ))}
            </div>
          )}
          {plan ? (
            <Badge
              variant='outline'
              className={cn(
                'inline-flex h-6 items-center gap-1 rounded-md px-2 font-mono text-[11px] font-semibold tracking-normal',
                plan.className
              )}
            >
              {plan.label}
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
              groups
                .filter((group) => group.windows.length > 0)
                .map((group) => (
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
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
