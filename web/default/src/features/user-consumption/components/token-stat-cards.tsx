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
import { useMemo } from 'react'
import { Layers, Hash, Zap, Coins } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import type { UserConsumptionSummary } from '../types'

interface TokenStatCardsProps {
  data: UserConsumptionSummary[]
  loading?: boolean
}

export function TokenStatCards({ data, loading }: TokenStatCardsProps) {
  const { t } = useTranslation()

  const stats = useMemo(() => {
    if (!data || data.length === 0) {
      return { totalTokens: 0, totalCount: 0, avgTokens: 0, totalQuota: 0 }
    }

    let totalTokens = 0
    let totalCount = 0
    let totalQuota = 0

    for (const item of data) {
      totalTokens += item.total_tokens
      totalCount += item.request_count
      totalQuota += item.quota
    }

    return {
      totalTokens,
      totalCount,
      avgTokens: totalCount > 0 ? Math.round(totalTokens / totalCount) : 0,
      totalQuota,
    }
  }, [data])

  const items = [
    {
      title: t('Total Tokens'),
      value: formatNumber(stats.totalTokens),
      desc: t('All tokens consumed'),
      icon: Layers,
    },
    {
      title: t('Total Requests'),
      value: formatNumber(stats.totalCount),
      desc: t('API calls made'),
      icon: Hash,
    },
    {
      title: t('Average Tokens'),
      value: formatNumber(stats.avgTokens),
      desc: t('Tokens per request'),
      icon: Zap,
    },
    {
      title: t('Total Quota'),
      value: formatNumber(stats.totalQuota),
      desc: t('Quota consumed'),
      icon: Coins,
    },
  ]

  if (loading) {
    return (
      <div className='overflow-hidden rounded-lg border'>
        <div className='divide-border/60 grid grid-cols-2 divide-x sm:grid-cols-4'>
          {items.map((it) => (
            <div key={it.title} className='px-3 py-2.5 sm:px-5 sm:py-4'>
              <div className='h-4 w-20 animate-pulse rounded bg-muted' />
              <div className='mt-2 h-7 w-16 animate-pulse rounded bg-muted' />
            </div>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid grid-cols-2 divide-x sm:grid-cols-4'>
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
              <div className='text-foreground mt-1.5 font-mono text-lg font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl'>
                {it.value}
              </div>
              <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
                {it.desc}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
