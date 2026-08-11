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
import { VChart } from '@visactor/react-vchart'
import { KeyRound } from 'lucide-react'
import { useMemo, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useTheme } from '@/context/theme-provider'
import { VCHART_OPTION } from '@/lib/vchart'

import { rankChartHeight } from '../lib/rank-axis'
import { processTokenConsumptionRankChartData } from '../lib/token-rank'
import type { UserConsumptionSummary } from '../types'

interface TokenConsumptionChartsProps {
  data: UserConsumptionSummary[]
  loading?: boolean
  limit?: number
  renderKey?: string
}

export function TokenConsumptionCharts({
  data,
  loading,
  limit = 15,
  renderKey = 'default',
}: TokenConsumptionChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const [themeReady, setThemeReady] = useState(false)

  useEffect(() => {
    const timer = setTimeout(() => setThemeReady(true), 100)
    return () => clearTimeout(timer)
  }, [resolvedTheme])

  const spec = useMemo(() => {
    return processTokenConsumptionRankChartData(data, t, limit)
  }, [data, limit, t])

  const itemCount = spec.data[0].values.length
  const hasData = itemCount > 0
  const chartHeight = rankChartHeight(itemCount)

  if (loading) {
    return (
      <div className='overflow-hidden rounded-lg border'>
        <div className='bg-muted h-[300px] animate-pulse sm:h-96' />
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex w-full items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
        <div className='flex items-center gap-2'>
          <KeyRound className='text-muted-foreground/60 size-4' />
          <div className='text-sm font-semibold'>
            {t('Token Consumption Ranking')}
          </div>
        </div>
      </div>

      <div className='p-1.5 sm:p-2' style={{ height: chartHeight }}>
        {themeReady && hasData && (
          <VChart
            key={`token-consumption-rank-${renderKey}-${limit}-${resolvedTheme}`}
            spec={{
              ...spec,
              theme: resolvedTheme === 'dark' ? 'dark' : 'light',
              background: 'transparent',
            }}
            option={VCHART_OPTION}
          />
        )}
        {!hasData && (
          <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
            {t('No consumption data found')}
          </div>
        )}
      </div>
    </div>
  )
}
