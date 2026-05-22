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
import { useEffect, useState } from 'react'
import { Layers, Hash, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { formatNumber } from '@/lib/format'
import { computeTimeRange } from '@/lib/time'
import { Skeleton } from '@/components/ui/skeleton'
import { getUserQuotaDates } from '@/features/dashboard/api'
import {
  buildQueryParams,
  calculateDashboardStats,
  getDefaultDays,
  safeDivide,
} from '@/features/dashboard/lib'
import type {
  QuotaDataItem,
  DashboardFilters,
} from '@/features/dashboard/types'

interface TokenStatCardsProps {
  filters?: DashboardFilters
  onDataUpdate?: (data: QuotaDataItem[], loading: boolean) => void
}

export function TokenStatCards(props: TokenStatCardsProps) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = !!(user?.role && user.role >= 10)
  const [stats, setStats] = useState<{
    totalTokens: number
    totalCount: number
  } | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [timeRangeMinutes, setTimeRangeMinutes] = useState(0)

  const { filters, onDataUpdate } = props

  useEffect(() => {
    const abortController = new AbortController()
    setLoading(true)
    setError(false)
    onDataUpdate?.([], true)

    const timeRange = computeTimeRange(
      getDefaultDays(filters?.time_granularity),
      filters?.start_timestamp,
      filters?.end_timestamp
    )
    const timeDiff = (timeRange.end_timestamp - timeRange.start_timestamp) / 60
    setTimeRangeMinutes(timeDiff)

    getUserQuotaDates(buildQueryParams(timeRange, filters), isAdmin)
      .then((res) => {
        if (abortController.signal.aborted) return
        const data = res?.data || []
        const result = calculateDashboardStats(data)
        setStats({
          totalTokens: result.totalTokens,
          totalCount: result.totalCount,
        })
        onDataUpdate?.(data, false)
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        setStats(null)
        setError(true)
        onDataUpdate?.([], false)
      })
      .finally(() => {
        if (!abortController.signal.aborted) {
          setLoading(false)
        }
      })

    return () => {
      abortController.abort()
    }
  }, [filters, isAdmin, onDataUpdate])

  const totalTokens = stats?.totalTokens ?? 0
  const totalCount = stats?.totalCount ?? 0
  const avgTokensPerCall =
    totalCount > 0 ? Math.round(totalTokens / totalCount) : 0
  const tpm = safeDivide(totalTokens, timeRangeMinutes)

  const items = [
    {
      title: t('Total Tokens'),
      value: formatNumber(totalTokens),
      desc: t('Statistical tokens'),
      icon: Layers,
    },
    {
      title: t('Average Tokens'),
      value: formatNumber(avgTokensPerCall),
      desc: t('Tokens per request'),
      icon: Hash,
    },
    {
      title: t('Tokens per minute'),
      value: formatNumber(Math.round(tpm)),
      desc: t('Tokens per minute'),
      icon: Zap,
    },
  ]

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid grid-cols-3 divide-x'>
        {items.map((it) => {
          const Icon = it.icon
          return (
            <div key={it.title} className='px-3 py-2.5 sm:px-5 sm:py-4'>
              <div className='flex items-center gap-2'>
                <Icon className='text-muted-foreground/60 size-3.5 shrink-0' />
                <div className='text-muted-foreground truncate text-xs font-medium tracking-wider uppercase'>
                  {it.title}
                </div>
              </div>

              {loading ? (
                <div className='mt-2 space-y-1.5'>
                  <Skeleton className='h-7 w-20' />
                  <Skeleton className='h-3.5 w-28' />
                </div>
              ) : error ? (
                <>
                  <div className='text-muted-foreground mt-1.5 font-mono text-lg font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl'>
                    --
                  </div>
                  <div className='text-muted-foreground/40 mt-1 hidden text-xs md:block'>
                    {it.desc}
                  </div>
                </>
              ) : (
                <>
                  <div className='text-foreground mt-1.5 font-mono text-lg font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl'>
                    {it.value}
                  </div>
                  <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
                    {it.desc}
                  </div>
                </>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
