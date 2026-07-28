import dayjs from '@/lib/dayjs'
import {
  formatNumber as formatSharedNumber,
  formatQuota,
  formatTokens,
} from '@/lib/format'

import type { ModelAvailabilityStatus } from '../types'

type ModelStatusVisual = {
  labelKey: string
  badgeClassName: string
  dotClassName: string
  barClassName: string
}

const MODEL_STATUS_VISUALS: Record<ModelAvailabilityStatus, ModelStatusVisual> =
  {
    normal: {
      labelKey: 'Normal',
      badgeClassName:
        'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
      dotClassName: 'bg-emerald-500',
      barClassName: 'bg-emerald-500',
    },
    warning: {
      labelKey: 'Warning',
      badgeClassName:
        'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300',
      dotClassName: 'bg-amber-500',
      barClassName: 'bg-amber-500',
    },
    error: {
      labelKey: 'Error',
      badgeClassName:
        'border-red-500/30 bg-red-500/10 text-red-700 dark:text-red-300',
      dotClassName: 'bg-red-500',
      barClassName: 'bg-red-500',
    },
    no_request: {
      labelKey: 'No requests',
      badgeClassName: 'border-border bg-muted text-muted-foreground',
      dotClassName: 'bg-muted-foreground/35',
      barClassName: 'bg-muted',
    },
  }

export function modelStatusVisual(
  status: ModelAvailabilityStatus
): ModelStatusVisual {
  return MODEL_STATUS_VISUALS[status] ?? MODEL_STATUS_VISUALS.no_request
}

export function canInspectModelStatusErrors(
  status: ModelAvailabilityStatus
): boolean {
  return status === 'warning' || status === 'error'
}

export function formatModelStatusPercent(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return Intl.NumberFormat(undefined, {
    style: 'percent',
    maximumFractionDigits: 1,
  }).format(value)
}

export function formatModelStatusNumber(value: number): string {
  return formatSharedNumber(value)
}

export function formatModelStatusMetric(value: number, digits = 1): string {
  if (!Number.isFinite(value)) return '-'
  return Intl.NumberFormat(undefined, {
    maximumFractionDigits: digits,
  }).format(value)
}

export function formatModelStatusMs(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '-'
  return `${formatSharedNumber(Math.round(value))}ms`
}

export function formatModelStatusTime(timestamp: number): string {
  if (!timestamp) return '-'
  return dayjs.unix(timestamp).format('HH:mm:ss')
}

export function formatModelStatusDateTime(timestamp: number): string {
  if (!timestamp) return '-'
  return dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm:ss')
}

export function formatModelStatusBucketTime(timestamp: number): string {
  if (!timestamp) return '-'
  return dayjs.unix(timestamp).format('MM/DD HH:mm')
}

export function formatModelStatusBucketRange(timestamp: number): string {
  if (!timestamp) return '-'

  const startTime = dayjs.unix(timestamp)
  const endTime = startTime.add(1, 'hour')
  return `${startTime.format('MM/DD HH:mm')} - ${endTime.format('MM/DD HH:mm')}`
}

export function calculateModelStatusSuccessCount(
  requestCount: number,
  successRate: number
): number {
  if (
    !Number.isFinite(requestCount) ||
    !Number.isFinite(successRate) ||
    requestCount <= 0 ||
    successRate <= 0
  ) {
    return 0
  }

  return Math.min(requestCount, Math.round(requestCount * successRate))
}

export { formatQuota, formatTokens }
