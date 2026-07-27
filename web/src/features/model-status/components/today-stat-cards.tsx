import { Coins, Gauge, Hash, Sigma, Timer } from 'lucide-react'
import type { ComponentType } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'

import {
  formatModelStatusMetric,
  formatModelStatusNumber,
  formatQuota,
  formatTokens,
} from '../lib/format'
import type { ModelStatusToday } from '../types'

type TodayStatCardsProps = {
  today: ModelStatusToday
}

type TodayStatItem = {
  label: string
  value: string
  icon: ComponentType<{ className?: string }>
}

export function TodayStatCards(props: TodayStatCardsProps) {
  const { t } = useTranslation()
  const statItems: TodayStatItem[] = [
    {
      label: t('Total Tokens'),
      value: formatTokens(props.today.tokens),
      icon: Sigma,
    },
    {
      label: t('Total Quota'),
      value: formatQuota(props.today.quota),
      icon: Coins,
    },
    {
      label: t('Requests'),
      value: formatModelStatusNumber(props.today.request_count),
      icon: Hash,
    },
    {
      label: t('Average RPM'),
      value: formatModelStatusMetric(props.today.rpm, 3),
      icon: Timer,
    },
    {
      label: t('Average TPM'),
      value: formatModelStatusMetric(props.today.tpm, 3),
      icon: Gauge,
    },
  ]

  return (
    <section className='space-y-3'>
      <div className='flex items-start gap-3 px-1'>
        <div className='text-primary pt-1'>
          <Gauge className='size-4' aria-hidden='true' />
        </div>
        <div>
          <div className='flex flex-wrap items-center gap-2'>
            <h2 className='font-semibold'>{t('Today Token Stats')}</h2>
            <Badge
              variant='secondary'
              className='text-primary rounded-full px-2'
            >
              {t('Global')}
            </Badge>
          </div>
          <p className='text-muted-foreground text-sm'>
            {t('Aggregated from 00:00 today. No login required.')}
          </p>
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'>
        {statItems.map((item) => (
          <Card
            key={item.label}
            className='border-foreground/80 bg-background px-4 py-4 shadow-none'
          >
            <div className='flex items-center gap-3'>
              <div className='border-foreground/80 flex size-8 items-center justify-center rounded-lg border'>
                <item.icon className='size-4' aria-hidden='true' />
              </div>
              <span className='text-muted-foreground text-sm'>{item.label}</span>
            </div>
            <div className='mt-3 truncate text-2xl font-bold tracking-tight'>
              {item.value}
            </div>
          </Card>
        ))}
      </div>
    </section>
  )
}
