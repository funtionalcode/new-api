import { useQuery } from '@tanstack/react-query'
import { AlertCircle } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { Card } from '@/components/ui/card'

import { getModelStatus } from './api'
import { ModelAvailabilitySection } from './components/model-availability-section'
import { ModelStatusLoading } from './components/model-status-loading'
import { StatusOverviewCard } from './components/status-overview-card'
import { TodayStatCards } from './components/today-stat-cards'

const MODEL_STATUS_REFRESH_INTERVAL_MS = 60_000

export function ModelStatus() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['model-status'],
    queryFn: getModelStatus,
    refetchInterval: MODEL_STATUS_REFRESH_INTERVAL_MS,
    staleTime: 30_000,
  })
  const snapshot = query.data?.data
  let content: ReactNode

  if (query.isLoading) {
    content = <ModelStatusLoading />
  } else if (!snapshot) {
    content = (
      <ModelStatusError
        message={
          query.error instanceof Error
            ? query.error.message
            : t('Unable to load model status')
        }
      />
    )
  } else {
    content = (
      <>
        <StatusOverviewCard
          overview={snapshot.overview}
          isRefreshing={query.isFetching}
          onRefresh={() => {
            void query.refetch()
          }}
        />
        <TodayStatCards today={snapshot.today} />
        <ModelAvailabilitySection models={snapshot.models} />
      </>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <div className='relative'>
        <div
          aria-hidden
          className='pointer-events-none absolute inset-x-0 top-0 h-[520px] opacity-20 dark:opacity-[0.10]'
          style={{
            background: [
              'radial-gradient(ellipse 60% 50% at 20% 15%, oklch(0.72 0.18 250 / 80%) 0%, transparent 70%)',
              'radial-gradient(ellipse 50% 40% at 80% 10%, oklch(0.65 0.15 200 / 60%) 0%, transparent 70%)',
            ].join(', '),
            maskImage:
              'linear-gradient(to bottom, black 35%, transparent 100%)',
            WebkitMaskImage:
              'linear-gradient(to bottom, black 35%, transparent 100%)',
          }}
        />
        <PageTransition className='relative mx-auto w-full max-w-[1280px] space-y-8 px-3 pt-16 pb-10 sm:px-6 sm:pt-20 sm:pb-12 xl:px-8'>
          {content}
        </PageTransition>
      </div>
    </PublicLayout>
  )
}

function ModelStatusError(props: { message: string }) {
  const { t } = useTranslation()
  return (
    <Card className='border-dashed px-6 py-12 text-center shadow-none'>
      <AlertCircle
        className='text-destructive mx-auto mb-3 size-8'
        aria-hidden='true'
      />
      <h2 className='text-base font-semibold'>
        {t('Unable to load model status')}
      </h2>
      <p className='text-muted-foreground mx-auto mt-2 max-w-md text-sm'>
        {props.message}
      </p>
    </Card>
  )
}
