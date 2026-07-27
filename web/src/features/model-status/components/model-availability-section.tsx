import { Radio } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import {
  calculateModelStatusSuccessCount,
  formatModelStatusBucketRange,
  formatModelStatusBucketTime,
  formatModelStatusMetric,
  formatModelStatusMs,
  formatModelStatusNumber,
  formatModelStatusPercent,
  modelStatusVisual,
} from '../lib/format'
import {
  getModelStatusGroups,
  getModelStatusModelMeta,
  type ModelStatusModelGroup,
} from '../lib/model-metadata'
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
  const groups = getModelStatusGroups(props.models)

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
        <TooltipProvider delay={0}>
          <div className='space-y-6'>
            {groups.map((group) => (
              <ModelAvailabilityGroup key={group.key} group={group} />
            ))}
          </div>
        </TooltipProvider>
      )}
    </section>
  )
}

function ModelAvailabilityGroup(props: { group: ModelStatusModelGroup }) {
  const { t } = useTranslation()

  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3 px-1'>
        <div className='flex min-w-0 items-center gap-2'>
          <div className='border-border bg-background flex size-7 shrink-0 items-center justify-center rounded-lg border shadow-xs'>
            {getLobeIcon(props.group.iconKey, 18)}
          </div>
          <h3 className='truncate text-sm font-semibold'>
            {getModelStatusGroupLabel(props.group.label, t)}
          </h3>
        </div>
        <Badge
          variant='secondary'
          className='text-muted-foreground h-5 rounded-full px-2 text-[11px]'
        >
          {formatModelStatusNumber(props.group.models.length)}
        </Badge>
      </div>

      <div className='grid gap-4 lg:grid-cols-2'>
        {props.group.models.map((model) => (
          <ModelAvailabilityCard key={model.model_name} model={model} />
        ))}
      </div>
    </div>
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
    <div className='text-muted-foreground flex flex-wrap gap-4 text-xs'>
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
  const meta = getModelStatusModelMeta(props.model.model_name)

  return (
    <Card className='border-border bg-background hover:shadow-foreground/10 min-h-40 px-4 py-4 shadow-none transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg sm:px-5'>
      <div className='flex items-start justify-between gap-4'>
        <div className='flex min-w-0 items-start gap-3'>
          <div className='border-foreground/80 bg-background flex size-10 shrink-0 items-center justify-center rounded-xl border shadow-xs'>
            {getLobeIcon(meta.iconKey, 24)}
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
                {t('Requests')}{' '}
                {formatModelStatusNumber(props.model.request_count)}
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
          <div className='text-muted-foreground text-xs'>
            {t('Success rate')}
          </div>
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
  const successCount = calculateModelStatusSuccessCount(
    props.bucket.request_count,
    props.bucket.success_rate
  )
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
    <Tooltip>
      <TooltipTrigger
        render={
          <div
            tabIndex={0}
            role='img'
            aria-label={title}
            className={cn(
              'min-w-1 flex-1 rounded-sm transition-all hover:-translate-y-0.5 hover:opacity-90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none',
              props.bucket.request_count > 0 ? 'h-9' : 'h-2',
              visual.barClassName
            )}
          />
        }
      />
      <TooltipContent
        side='top'
        sideOffset={8}
        className='grid w-[174px] max-w-none gap-1 rounded-md bg-zinc-700 px-3 py-2 text-xs text-white shadow-xl ring-0 [&>svg]:bg-zinc-700 [&>svg]:fill-zinc-700'
      >
        <div className='border-b border-white/25 pb-1 font-semibold tabular-nums'>
          {formatModelStatusBucketRange(props.bucket.start)}
        </div>
        <TimelineTooltipRow
          label={t('Requests')}
          value={formatModelStatusNumber(props.bucket.request_count)}
        />
        <TimelineTooltipRow
          label={t('Successful requests')}
          value={formatModelStatusNumber(successCount)}
        />
        <TimelineTooltipRow
          label={t('Success rate')}
          value={formatModelStatusPercent(props.bucket.success_rate)}
        />
        <TimelineTooltipRow label={t('Status')} value={t(visual.labelKey)} />
      </TooltipContent>
    </Tooltip>
  )
}

function TimelineTooltipRow(props: { label: string; value: string }) {
  return (
    <div className='grid grid-cols-[1fr_auto] items-center gap-3'>
      <span className='text-white/75'>{props.label}</span>
      <span className='font-semibold tabular-nums'>{props.value}</span>
    </div>
  )
}

function getModelStatusGroupLabel(
  label: string,
  t: (key: string) => string
): string {
  if (label === 'Image models' || label === 'Other models') {
    return t(label)
  }

  return label
}
