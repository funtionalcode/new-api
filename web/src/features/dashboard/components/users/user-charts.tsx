/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { Users } from 'lucide-react'
import {
  useEffect,
  useMemo,
  useState,
  useRef,
  useCallback,
} from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useTheme } from '@/context/theme-provider'
import { getUserQuotaDataByUsers } from '@/features/dashboard/api'
import { TimeRangeControls } from '@/features/dashboard/components/time-range-controls'
import {
  getDefaultDays,
  saveGranularity,
  processUserChartData,
  processUserModelUsageChartData,
} from '@/features/dashboard/lib'
import {
  buildUserChartTimeRangeDates,
  buildUserChartTimeRange,
  formatUserChartTimeRangeLabel,
} from '@/features/dashboard/lib/user-chart-time-range'
import type {
  ProcessedUserChartData,
  UserChartMetric,
  UserChartsFilters,
} from '@/features/dashboard/types'
import type { TimeGranularity } from '@/lib/time'
import { VCHART_OPTION } from '@/lib/vchart'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

const USER_CHARTS: {
  value: string
  labelKey: string
  specKey: keyof ProcessedUserChartData
}[] = [
  {
    value: 'rank',
    labelKey: 'User Consumption Ranking',
    specKey: 'spec_user_rank',
  },
  {
    value: 'trend',
    labelKey: 'User Consumption Trend',
    specKey: 'spec_user_trend',
  },
]

const TOP_USER_LIMIT_OPTIONS = [5, 10, 20, 50]

interface UserChartsProps {
  filters: UserChartsFilters
  onFiltersChange: (filters: UserChartsFilters) => void
}

export function UserCharts(props: UserChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  // The selection is owned by the dashboard parent so it persists across
  // sub-section switches; the rolling window is derived from the chosen range.
  const timeGranularity = props.filters.timeGranularity
  const selectedRange = props.filters.selectedRange
  const topUserLimit = props.filters.topUserLimit
  const metric = props.filters.metric
  const onFiltersChange = props.onFiltersChange

  const timeRangeDates = useMemo(() => {
    return buildUserChartTimeRangeDates(props.filters)
  }, [props.filters])

  const timeRange = useMemo(() => {
    return buildUserChartTimeRange(props.filters)
  }, [props.filters])

  const timeRangeLabel = useMemo(
    () => formatUserChartTimeRangeLabel(timeRange),
    [timeRange]
  )
  const chartRenderKey = `${timeRange.start_timestamp}-${timeRange.end_timestamp}-${timeGranularity}`

  const handleRangeChange = useCallback(
    (days: number) => {
      onFiltersChange({
        ...props.filters,
        selectedRange: days,
        customStartTime: undefined,
        customEndTime: undefined,
      })
    },
    [onFiltersChange, props.filters]
  )

  const handleGranularityChange = useCallback(
    (g: TimeGranularity) => {
      saveGranularity(g)
      onFiltersChange({
        ...props.filters,
        timeGranularity: g,
        selectedRange: getDefaultDays(g),
        customStartTime: undefined,
        customEndTime: undefined,
      })
    },
    [onFiltersChange, props.filters]
  )

  const handleCustomStartChange = useCallback(
    (date: Date | undefined) => {
      onFiltersChange({
        ...props.filters,
        customStartTime: date,
        customEndTime: date
          ? (props.filters.customEndTime ?? timeRangeDates.end)
          : props.filters.customEndTime,
      })
    },
    [onFiltersChange, props.filters, timeRangeDates.end]
  )

  const handleCustomEndChange = useCallback(
    (date: Date | undefined) => {
      onFiltersChange({
        ...props.filters,
        customStartTime: date
          ? (props.filters.customStartTime ?? timeRangeDates.start)
          : props.filters.customStartTime,
        customEndTime: date,
      })
    },
    [onFiltersChange, props.filters, timeRangeDates.start]
  )

  const handleTopUserLimitChange = useCallback(
    (limit: number) => {
      onFiltersChange({ ...props.filters, topUserLimit: limit })
    },
    [onFiltersChange, props.filters]
  )

  const handleMetricChange = useCallback(
    (nextMetric: UserChartMetric) => {
      onFiltersChange({ ...props.filters, metric: nextMetric })
    },
    [onFiltersChange, props.filters]
  )

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      themeManagerRef.current = ThemeManager
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    updateTheme()
  }, [resolvedTheme])

  const { data: userData, isLoading } = useQuery({
    queryKey: ['dashboard', 'user-quota', timeRange],
    queryFn: () => getUserQuotaDataByUsers(timeRange),
    select: (res) => (res.success ? res.data : []),
    staleTime: 60_000,
  })

  const chartData = useMemo(
    () =>
      processUserChartData(
        isLoading ? [] : (userData ?? []),
        timeGranularity,
        t,
        topUserLimit,
        metric
      ),
    [userData, isLoading, timeGranularity, t, topUserLimit, metric]
  )
  const modelUsageChartData = useMemo(
    () =>
      processUserModelUsageChartData(
        isLoading ? [] : (userData ?? []),
        t,
        topUserLimit,
        metric
      ),
    [userData, isLoading, t, topUserLimit, metric]
  )
  const modelUsageChartHeight = useMemo(() => {
    const userCount = modelUsageChartData.userCount
    return Math.max(360, Math.min(userCount * 56 + 140, 1200))
  }, [modelUsageChartData.userCount])

  return (
    <div className='space-y-3'>
      <TimeRangeControls
        endTime={timeRangeDates.end}
        loading={isLoading}
        onEndTimeChange={handleCustomEndChange}
        onGranularityChange={handleGranularityChange}
        onRangeChange={handleRangeChange}
        onStartTimeChange={handleCustomStartChange}
        rangeLabel={timeRangeLabel}
        selectedRange={selectedRange}
        startTime={timeRangeDates.start}
        timeGranularity={timeGranularity}
      >
        <Tabs
          value={metric}
          onValueChange={(value) =>
            handleMetricChange(value as UserChartMetric)
          }
          className='shrink-0'
        >
          <TabsList>
            <TabsTrigger value='tokens' className='px-2.5 text-xs'>
              {t('Tokens')}
            </TabsTrigger>
            <TabsTrigger value='amount' className='px-2.5 text-xs'>
              {t('Amount')}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <Tabs
          value={String(topUserLimit)}
          onValueChange={(value) => handleTopUserLimitChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            <span className='text-muted-foreground px-2 text-xs font-medium whitespace-nowrap'>
              {t('Top Users')}
            </span>
            {TOP_USER_LIMIT_OPTIONS.map((limit) => (
              <TabsTrigger
                key={limit}
                value={String(limit)}
                className='px-2.5 text-xs'
              >
                {t('Top {{count}}', { count: limit })}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </TimeRangeControls>

      <div className='grid gap-3'>
        {USER_CHARTS.map((chart) => {
          const spec = chartData[chart.specKey]

          return (
            <div
              key={chart.value}
              className='overflow-hidden rounded-lg border'
            >
              <div className='flex w-full items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
                <IconBadge tone='info' size='sm'>
                  <Users />
                </IconBadge>
                <div className='text-sm font-semibold'>{t(chart.labelKey)}</div>
              </div>

              <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
                {isLoading ? (
                  <Skeleton className='h-full w-full' />
                ) : (
                  themeReady &&
                  spec && (
                    <VChart
                      key={`user-${chart.value}-${chartRenderKey}-${metric}-${topUserLimit}-${resolvedTheme}`}
                      spec={{
                        ...spec,
                        theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                        background: 'transparent',
                      }}
                      option={VCHART_OPTION}
                    />
                  )
                )}
              </div>
            </div>
          )
        })}

        <div className='overflow-hidden rounded-lg border'>
          <div className='flex w-full items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
            <IconBadge tone='info' size='sm'>
              <Users />
            </IconBadge>
            <div className='text-sm font-semibold'>
              {t('User Model Usage Details')}
            </div>
          </div>

          <div className='max-h-[760px] overflow-y-auto p-1.5 sm:p-2'>
            <div style={{ height: modelUsageChartHeight }}>
              {isLoading ? (
                <Skeleton className='h-full w-full' />
              ) : (
                themeReady && (
                  <VChart
                    key={`user-model-usage-${chartRenderKey}-${metric}-${topUserLimit}-${resolvedTheme}`}
                    spec={{
                      ...modelUsageChartData.spec_user_model_usage,
                      theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                      background: 'transparent',
                    }}
                    option={VCHART_OPTION}
                  />
                )
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
