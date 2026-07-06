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
import { useMemo, useState, useEffect } from 'react'
import { VChart } from '@visactor/react-vchart'
import { Activity, PieChart, BarChart3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useTheme } from '@/context/theme-provider'
import { formatTokens } from '@/lib/format'
import { VCHART_OPTION } from '@/lib/vchart'
import type { UserConsumptionSummary } from '../types'

type ChartTab = 'trend' | 'proportion' | 'top'

const CHART_OPTIONS: { value: ChartTab; labelKey: string; icon: typeof Activity }[] = [
  { value: 'trend', labelKey: 'Trend', icon: Activity },
  { value: 'proportion', labelKey: 'Distribution', icon: PieChart },
  { value: 'top', labelKey: 'Top Tokens', icon: BarChart3 },
]

interface TokenConsumptionChartsProps {
  data: UserConsumptionSummary[]
  loading?: boolean
}

export function TokenConsumptionCharts({ data, loading }: TokenConsumptionChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const [activeTab, setActiveTab] = useState<ChartTab>('trend')
  const [themeReady, setThemeReady] = useState(false)

  useEffect(() => {
    const timer = setTimeout(() => setThemeReady(true), 100)
    return () => clearTimeout(timer)
  }, [resolvedTheme])

  const aggregatedData = useMemo(() => {
    if (!data || data.length === 0) return { byToken: [], totalTokens: 0 }

    const tokenMap = new Map<string, { tokens: number; requests: number; quota: number }>()
    let totalTokens = 0

    for (const item of data) {
      const key = item.token_name || `Token #${item.token_id}`
      const existing = tokenMap.get(key) || { tokens: 0, requests: 0, quota: 0 }
      existing.tokens += item.total_tokens
      existing.requests += item.request_count
      existing.quota += item.quota
      tokenMap.set(key, existing)
      totalTokens += item.total_tokens
    }

    const byToken = [...tokenMap.entries()]
      .map(([name, stats]) => ({ name, ...stats }))
      .sort((a, b) => b.tokens - a.tokens)

    return { byToken, totalTokens }
  }, [data])

  const spec = useMemo(() => {
    const { byToken } = aggregatedData
    const topN = 15
    const topItems = byToken.slice(0, topN)
    const otherTokens = byToken.slice(topN).reduce((sum, item) => sum + item.tokens, 0)
    const chartData = otherTokens > 0
      ? [...topItems, { name: t('Other'), tokens: otherTokens, requests: 0, quota: 0 }]
      : topItems

    if (activeTab === 'trend') {
      return {
        type: 'area',
        data: [{ values: chartData.map((item) => ({ x: item.name, y: item.tokens, series: 'Tokens' })) }],
        xField: 'x',
        yField: 'y',
        seriesField: 'series',
        color: ['var(--chart-1)'],
        area: { visible: true, opacity: 0.3 },
        line: { visible: true, style: { lineWidth: 2 } },
        point: { visible: false },
        axes: [
          { orient: 'bottom', label: { visible: true, style: { angle: -45, textAlign: 'right' } } },
          { orient: 'left', label: { visible: true, formatMethod: (val: number) => formatTokens(val) } },
        ],
        tooltip: {
          mark: {
            content: [
              { key: t('Tokens'), value: (datum: Record<string, number>) => Number(datum.y || 0).toLocaleString() },
            ],
          },
        },
      }
    }

    if (activeTab === 'proportion') {
      return {
        type: 'pie',
        data: [{ values: chartData.map((item) => ({ type: item.name, value: item.tokens })) }],
        valueField: 'value',
        categoryField: 'type',
        label: { visible: true, position: 'outside', formatMethod: (val: number) => val.toLocaleString() },
        tooltip: {
          mark: {
            content: [
              { key: t('Tokens'), value: (datum: Record<string, number>) => Number(datum.value || 0).toLocaleString() },
            ],
          },
        },
      }
    }

    return {
      type: 'bar',
      data: [{ values: chartData.map((item) => ({ x: item.name, y: item.tokens })) }],
      xField: 'x',
      yField: 'y',
      color: ['var(--chart-2)'],
      bar: { style: { cornerRadius: [4, 4, 0, 0] } },
        axes: [
          { orient: 'bottom', label: { visible: true, style: { angle: -45, textAlign: 'right' } } },
          { orient: 'left', label: { visible: true, formatMethod: (val: number) => formatTokens(val) } },
        ],
      tooltip: {
        mark: {
          content: [
              { key: t('Tokens'), value: (datum: Record<string, number>) => Number(datum.y || 0).toLocaleString() },
          ],
        },
      },
    }
  }, [aggregatedData, activeTab, t])

  if (loading) {
    return (
      <div className='overflow-hidden rounded-lg border'>
        <div className='h-[300px] animate-pulse bg-muted' />
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex w-full flex-col gap-1.5 border-b px-3 py-2 sm:gap-3 sm:px-5 sm:py-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='flex items-center gap-2'>
          <Activity className='text-muted-foreground/60 size-4' />
          <div className='text-sm font-semibold'>
            {t('Token Consumption')}
          </div>
          <span className='text-muted-foreground text-xs'>
            {t('Total:')} {formatTokens(aggregatedData.totalTokens)}
          </span>
        </div>

        <div className='bg-muted/60 inline-flex h-7 w-full overflow-x-auto rounded-lg border p-0.5 sm:h-8 sm:w-auto'>
          {CHART_OPTIONS.map((tab) => (
            <button
              key={tab.value}
              type='button'
              onClick={() => setActiveTab(tab.value)}
              className={`shrink-0 rounded-md px-3 text-xs font-medium transition-colors ${
                activeTab === tab.value
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              {t(tab.labelKey)}
            </button>
          ))}
        </div>
      </div>

      <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
        {themeReady && aggregatedData.byToken.length > 0 && (
          <VChart
            key={`token-chart-${activeTab}-${resolvedTheme}`}
            spec={{
              ...spec,
              theme: resolvedTheme === 'dark' ? 'dark' : 'light',
              background: 'transparent',
            }}
            option={VCHART_OPTION}
          />
        )}
        {aggregatedData.byToken.length === 0 && (
          <div className='flex h-full items-center justify-center text-muted-foreground text-sm'>
            {t('No consumption data found')}
          </div>
        )}
      </div>
    </div>
  )
}
