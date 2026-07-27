import { Bot, Radio } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/utils'

import {
  formatModelStatusBucketTime,
  formatModelStatusMetric,
  formatModelStatusMs,
  formatModelStatusNumber,
  formatModelStatusPercent,
  modelStatusVisual,
} from '../lib/format'
import type {
  ModelAvailabilityStatus,
  ModelStatusBucket,
  ModelStatusModel,
} from '../types'

type ModelAvailabilitySectionProps = {
  models: ModelStatusModel[]
}

export function ModelAvailabilitySection(props: ModelAvailabilitySectionProps) {
  const { t } = useTranslation()

  return (
    <section className='space-y-4'>
      <div className='flex flex-col gap-3 px-1 lg:flex-row lg:items-end lg:justify-between'>
        <div className='flex items-start gap-3'>
          <div className='border-border bg-muted/50 flex size-8 items-center justify-center rounded-lg border'>
            <Radio className='size-4' aria-hidden='true' />
          </div>
          <div>
            <h2 className='font-semibold'>{t('Model Availability')}</h2>
            <p className='text-muted-foreground text-sm'>
              {t('Green ≥95%, yellow 80–95%, red <80%.')}
            </p>
          </div>
        </div>
        <StatusLegend />
      </div>

      {props.models.length === 0 ? (
        <Card className='border-dashed px-6 py-12 text-center shadow-none'>
          <p className='text-muted-foreground text-sm'>
            {t('No model traffic in the last 24 hours.')}
          </p>
        </Card>
      ) : (
        <div className='grid gap-4 lg:grid-cols-2'>
          {props.models.map((model) => (
            <ModelAvailabilityCard key={model.model_name} model={model} />
          ))}
        </div>
      )}
    </section>
  )
}

function StatusLegend() {
  const { t } = useTranslation()
  const items: Array<{ status: ModelAvailabilityStatus; label: string }> = [
    { status: 'normal', label: t('Success rate ≥95%') },
    { status: 'warning', label: t('Success rate 80–95%') },
    { status: 'error', label: t('Success rate <80%') },
    { status: 'no_request', label: t('No requests') },
  ]

  return (
    <div className='flex flex-wrap gap-4 text-xs text-muted-foreground'>
      {items.map((item) => {
        const visual = modelStatusVisual(item.status)
        return (
          <span key={item.status} className='inline-flex items-center gap-1.5'>
            <span className={cn('size-2 rounded-sm', visual.barClassName)} />
            {item.label}
          </span>
        )
      })}
    </div>
  )
}

function ModelAvailabilityCard(props: { model: ModelStatusModel }) {
  const { t } = useTranslation()
  const visual = modelStatusVisual(props.model.status)

  return (
    <Card className='border-border bg-background px-4 py-4 shadow-none sm:px-5'>
      <div className='flex items-start justify-between gap-4'>
        <div className='flex min-w-0 items-start gap-3'>
          <div className='border-foreground/80 flex size-10 shrink-0 items-center justify-center rounded-xl border'>
            <Bot className='size-5' aria-hidden='true' />
          </div>
          <div className='min-w-0'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <h3 className='truncate text-base font-semibold'>
                {props.model.model_name}
              </h3>
              <Badge variant='outline' className={visual.badgeClassName}>
                {t(visual.labelKey)}
              </Badge>
            </div>
            <div className='text-muted-foreground mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs'>
              <span>
                {t('Requests')} {formatModelStatusNumber(props.model.request_count)}
              </span>
              <span>
                {t('Avg first token')}{' '}
                {formatModelStatusMs(props.model.avg_first_response_time_ms)}
              </span>
              <span>
                {t('Latency')} {formatModelStatusMs(props.model.avg_latency_ms)}
              </span>
              <span>
                {t('TPS')} {formatModelStatusMetric(props.model.tps, 1)}
              </span>
            </div>
          </div>
        </div>
        <div className='shrink-0 text-right'>
          <div className='text-xl font-bold tracking-tight'>
            {formatModelStatusPercent(props.model.success_rate)}
          </div>
          <div className='text-muted-foreground text-xs'>{t('Success rate')}</div>
        </div>
      </div>

      <div className='mt-5'>
        <div className='flex h-10 items-end gap-1'>
          {props.model.buckets.map((bucket) => (
            <TimelineBar key={bucket.start} bucket={bucket} />
          ))}
        </div>
        <div className='text-muted-foreground mt-2 flex justify-between text-xs'>
          <span>
            {formatModelStatusBucketTime(props.model.buckets[0]?.start ?? 0)}
          </span>
          <span>{t('Last 24 hours')}</span>
          <span>
            {formatModelStatusBucketTime(
              props.model.buckets.at(-1)?.start ?? 0
            )}
          </span>
        </div>
      </div>
    </Card>
  )
}

function TimelineBar(props: { bucket: ModelStatusBucket }) {
  const { t } = useTranslation()
  const visual = modelStatusVisual(props.bucket.status)
  const title =
    props.bucket.request_count > 0
      ? t('{{time}} · {{count}} requests · {{rate}} success', {
          time: formatModelStatusBucketTime(props.bucket.start),
          count: formatModelStatusNumber(props.bucket.request_count),
          rate: formatModelStatusPercent(props.bucket.success_rate),
        })
      : t('{{time}} · No requests', {
          time: formatModelStatusBucketTime(props.bucket.start),
        })

  return (
    <div
      title={title}
      aria-label={title}
      className={cn(
        'min-w-1 flex-1 rounded-sm transition-opacity hover:opacity-80',
        props.bucket.request_count > 0 ? 'h-9' : 'h-2',
        visual.barClassName
      )}
    />
  )
}
