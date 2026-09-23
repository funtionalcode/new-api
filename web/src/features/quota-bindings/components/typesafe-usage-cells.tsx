import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'

import { Dialog } from '@/components/dialog'
import { EmptyState } from '@/components/empty-state'
import { Button } from '@/components/ui/button'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { TableCell } from '@/components/ui/table'
import { toIntlLocale } from '@/i18n/languages'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatCompactNumber, formatNumber } from '@/lib/format'

import { summarizeTypeSafeUsage } from '../lib/typesafe-usage'
import type { TypeSafeUsageBinding, TypeSafeUsageBucket } from '../types'

export function TypeSafeUsageCells(props: { binding: TypeSafeUsageBinding }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [open, setOpen] = useState(false)
  const [days, setDays] = useState(7)
  const [traffic, setTraffic] = useState('all')
  const [hourly, setHourly] = useState(true)
  const buckets = useMemo<TypeSafeUsageBucket[]>(() => {
    if (!props.binding.last_buckets) return []
    try {
      return JSON.parse(props.binding.last_buckets) as TypeSafeUsageBucket[]
    } catch {
      return []
    }
  }, [props.binding.last_buckets])
  const summary = summarizeTypeSafeUsage(
    buckets,
    days,
    traffic,
    hourly,
    Date.now()
  )
  const keys = new Map<string, string>()
  for (const bucket of buckets) {
    if (bucket.apiKeyId) {
      keys.set(bucket.apiKeyId, bucket.apiKeyName || bucket.apiKeyId)
    }
  }
  const config = {
    input: { label: t('Input Tokens'), color: 'var(--chart-1)' },
    output: { label: t('Output Tokens'), color: 'var(--chart-2)' },
    requests: { label: t('API Requests'), color: 'var(--chart-1)' },
    spend: { label: t('Estimated Cost'), color: 'var(--chart-1)' },
  }
  return (
    <TableCell>
      <Button
        variant='outline'
        disabled={!props.binding.last_refreshed_at}
        onClick={() => setOpen(true)}
      >
        {t('View Usage')}
      </Button>
      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={`${t('TypeSafe Usage')} · ${props.binding.name}`}
        description={t(
          'Statistics may be delayed. Estimated input cost: $0.042 per million tokens; output is free.'
        )}
        contentClassName='sm:max-w-5xl'
      >
        <div className='space-y-5'>
          <div className='flex flex-wrap gap-3'>
            <NativeSelect
              aria-label={t('Traffic')}
              value={traffic}
              onChange={(event) => setTraffic(event.target.value)}
            >
              <NativeSelectOption value='all'>
                {t('All traffic')}
              </NativeSelectOption>
              <NativeSelectOption value='api'>
                {t('API Keys')}
              </NativeSelectOption>
              <NativeSelectOption value='playground'>
                {t('Playground')}
              </NativeSelectOption>
              {[...keys].map(([id, name]) => (
                <NativeSelectOption key={id} value={`key:${id}`}>
                  {name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
            <NativeSelect
              aria-label={t('Time Range')}
              value={days}
              onChange={(event) => setDays(Number(event.target.value))}
            >
              <NativeSelectOption value={7}>
                {t('Last 7 days')}
              </NativeSelectOption>
              <NativeSelectOption value={30}>
                {t('Last 30 days')}
              </NativeSelectOption>
            </NativeSelect>
            <NativeSelect
              aria-label={t('Granularity')}
              value={hourly ? 'hour' : 'day'}
              onChange={(event) => setHourly(event.target.value === 'hour')}
            >
              <NativeSelectOption value='hour'>
                {t('Hourly')}
              </NativeSelectOption>
              <NativeSelectOption value='day'>{t('Daily')}</NativeSelectOption>
            </NativeSelect>
          </div>
          <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
            <div>
              {t('Estimated Cost')}
              <p className='text-xl font-semibold'>
                {formatBillingCurrencyFromUSD(summary.spend, {
                  digitsSmall: 4,
                  digitsLarge: 4,
                  locale,
                })}
              </p>
            </div>
            <div>
              {t('Total Tokens')}
              <p className='text-xl font-semibold'>
                {formatNumber(summary.input + summary.output, locale)}
              </p>
            </div>
            <div>
              {t('API Requests')}
              <p className='text-xl font-semibold'>
                {formatNumber(summary.requests, locale)}
              </p>
            </div>
          </div>
          {summary.points.length === 0 ? (
            <EmptyState />
          ) : (
            (['spend', 'input', 'requests'] as const).map((metric) => (
              <section key={metric}>
                <h3 className='mb-2 font-medium'>
                  {metric === 'input'
                    ? t('Total Tokens')
                    : config[metric].label}
                </h3>
                <ChartContainer config={config} className='h-48 w-full'>
                  <BarChart data={summary.points} accessibilityLayer>
                    <CartesianGrid vertical={false} />
                    <XAxis
                      dataKey='time'
                      minTickGap={35}
                      tickFormatter={(value: number) =>
                        new Date(value).toLocaleDateString(locale, {
                          month: 'short',
                          day: 'numeric',
                          ...(hourly ? { hour: 'numeric' } : {}),
                        })
                      }
                    />
                    <YAxis
                      tickFormatter={(value: number) =>
                        metric === 'spend'
                          ? formatBillingCurrencyFromUSD(value, {
                              digitsSmall: 4,
                              locale,
                            })
                          : formatCompactNumber(value, locale)
                      }
                    />
                    <ChartTooltip
                      content={
                        <ChartTooltipContent
                          labelFormatter={(value) =>
                            new Date(Number(value)).toLocaleString(locale)
                          }
                          formatter={(value, name) => (
                            <span>
                              {config[name as keyof typeof config]?.label}:{' '}
                              {metric === 'spend'
                                ? formatBillingCurrencyFromUSD(Number(value), {
                                    digitsSmall: 4,
                                    locale,
                                  })
                                : formatNumber(Number(value), locale)}
                            </span>
                          )}
                        />
                      }
                    />
                    <Bar
                      dataKey={metric}
                      stackId='usage'
                      fill={`var(--color-${metric})`}
                      isAnimationActive={false}
                    />
                    {metric === 'input' && (
                      <Bar
                        dataKey='output'
                        stackId='usage'
                        fill='var(--color-output)'
                        isAnimationActive={false}
                      />
                    )}
                  </BarChart>
                </ChartContainer>
              </section>
            ))
          )}
        </div>
      </Dialog>
    </TableCell>
  )
}
