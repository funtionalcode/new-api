import { Activity, RefreshCw, Server } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/utils'

import {
  formatModelStatusNumber,
  formatModelStatusPercent,
  formatModelStatusTime,
  modelStatusVisual,
} from '../lib/format'
import type {
  ModelAvailabilityStatus,
  ModelStatusOverview,
} from '../types'

type StatusOverviewCardProps = {
  overview: ModelStatusOverview
  isRefreshing: boolean
  onRefresh: () => void
}

export function StatusOverviewCard(props: StatusOverviewCardProps) {
  const { t } = useTranslation()
  const overallStatus = getOverviewStatus(props.overview)
  const visual = modelStatusVisual(overallStatus)

  return (
    <Card className='border-foreground/80 bg-background/95 px-4 py-4 shadow-none backdrop-blur sm:px-6 sm:py-5'>
      <div className='flex flex-col gap-5'>
        <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
          <div className='space-y-4'>
            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant='outline' className={visual.badgeClassName}>
                <span className={cn('size-1.5 rounded-full', visual.dotClassName)} />
                {t(visual.labelKey)}
              </Badge>
              <Badge variant='secondary' className='text-primary rounded-full'>
                <span className='bg-primary size-1.5 rounded-full' />
                {t('Last 24 hours · auto refresh {{seconds}}s', {
                  seconds: props.overview.auto_refresh_seconds,
                })}
              </Badge>
            </div>
            <div className='flex items-center gap-3'>
              <div className='from-primary/80 to-primary/40 text-primary-foreground flex size-11 items-center justify-center rounded-xl bg-gradient-to-br shadow-lg shadow-primary/20'>
                <Server className='size-5' aria-hidden='true' />
              </div>
              <div>
                <h1 className='text-2xl font-bold tracking-tight'>
                  {t('Model Status')}
                </h1>
                <p className='text-muted-foreground mt-1 text-sm'>
                  {t(
                    'Track live model availability and success rate from the last 24 hours.'
                  )}{' '}
                  {t('Updated {{time}}', {
                    time: formatModelStatusTime(props.overview.updated_at),
                  })}
                </p>
              </div>
            </div>
          </div>
          <Button
            type='button'
            onClick={props.onRefresh}
            disabled={props.isRefreshing}
            className='self-start'
          >
            <RefreshCw
              className={cn('size-4', props.isRefreshing && 'animate-spin')}
              aria-hidden='true'
            />
            {t('Refresh')}
          </Button>
        </div>

        <div className='border-foreground/80 border-t pt-4'>
          <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
            <div className='flex flex-wrap gap-2'>
              <OverviewBadge
                label={t('Normal')}
                value={props.overview.normal_count}
                status='normal'
              />
              <OverviewBadge
                label={t('Warning')}
                value={props.overview.warning_count}
                status='warning'
              />
              <OverviewBadge
                label={t('Error')}
                value={props.overview.error_count}
                status='error'
              />
            </div>
            <div className='flex flex-wrap gap-x-6 gap-y-2 text-sm'>
              <OverviewMetric
                label={t('Monitored models')}
                value={formatModelStatusNumber(props.overview.model_count)}
              />
              <OverviewMetric
                label={t('Total requests')}
                value={formatModelStatusNumber(props.overview.request_count)}
              />
              <OverviewMetric
                label={t('Average success rate')}
                value={formatModelStatusPercent(
                  props.overview.avg_success_rate
                )}
              />
            </div>
          </div>
        </div>
      </div>
    </Card>
  )
}

function OverviewBadge(props: {
  label: string
  value: number
  status: ModelAvailabilityStatus
}) {
  const visual = modelStatusVisual(props.status)
  return (
    <Badge
      variant='outline'
      className={cn('h-7 gap-2 rounded-full px-3', visual.badgeClassName)}
    >
      <Activity className='size-3.5' aria-hidden='true' />
      {props.label}
      <span className='font-semibold'>{formatModelStatusNumber(props.value)}</span>
    </Badge>
  )
}

function OverviewMetric(props: { label: string; value: string }) {
  return (
    <div className='flex items-center gap-1.5'>
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='font-semibold'>{props.value}</span>
    </div>
  )
}

function getOverviewStatus(
  overview: ModelStatusOverview
): ModelAvailabilityStatus {
  if (overview.error_count > 0) return 'error'
  if (overview.warning_count > 0) return 'warning'
  return 'normal'
}
