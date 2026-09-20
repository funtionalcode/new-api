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
import { Database, RefreshCw, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'

import { getTypeSafeEvaluations } from './api'
import { TypeSafeBeforeAfter } from './components/before-after'
import { EvaluationDetailDialog } from './components/evaluation-detail-dialog'
import type { TypeSafeStage } from './types'

const PAGE_SIZE = 20

export function TypeSafeEvaluationsPage() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [modelName, setModelName] = useState('')
  const [requestId, setRequestId] = useState('')
  const [appliedModelName, setAppliedModelName] = useState('')
  const [appliedRequestId, setAppliedRequestId] = useState('')
  const [detail, setDetail] = useState<{
    requestId: string
    stage: TypeSafeStage
  } | null>(null)

  const query = useQuery({
    queryKey: [
      'typesafe-evaluations',
      page,
      appliedModelName,
      appliedRequestId,
    ],
    queryFn: () =>
      getTypeSafeEvaluations({
        p: page,
        page_size: PAGE_SIZE,
        model_name: appliedModelName,
        request_id: appliedRequestId,
      }),
  })

  const items = query.data?.data?.items ?? []
  const total = query.data?.data?.total ?? 0
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const canPrev = page > 1
  const canNext = page < pageCount

  const applyFilters = () => {
    setPage(1)
    setAppliedModelName(modelName.trim())
    setAppliedRequestId(requestId.trim())
  }

  const skeletons = useMemo(
    () =>
      Array.from({ length: 3 }, (_, index) => `typesafe-skeleton-${index + 1}`),
    []
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('TypeSafe evaluations')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => {
            void query.refetch()
          }}
          disabled={query.isFetching}
        >
          <RefreshCw
            className={query.isFetching ? 'animate-spin' : undefined}
            aria-hidden='true'
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-3'>
          <p className='text-muted-foreground text-sm'>
            {t('Review before and after answers for each request')}
          </p>
          <form
            className='flex flex-col gap-2 sm:flex-row'
            onSubmit={(event) => {
              event.preventDefault()
              applyFilters()
            }}
          >
            <Input
              value={modelName}
              onChange={(event) => setModelName(event.target.value)}
              placeholder={t('Evaluated model')}
              aria-label={t('Evaluated model')}
            />
            <Input
              value={requestId}
              onChange={(event) => setRequestId(event.target.value)}
              placeholder={t('Request ID')}
              aria-label={t('Request ID')}
            />
            <Button type='submit' variant='secondary'>
              <Search aria-hidden='true' />
              {t('Search')}
            </Button>
          </form>

          {query.isError ? (
            <Alert variant='destructive'>
              <AlertDescription>
                {query.error instanceof Error
                  ? query.error.message
                  : t('Failed')}
              </AlertDescription>
            </Alert>
          ) : null}

          {query.isLoading ? (
            <div className='space-y-3'>
              {skeletons.map((id) => (
                <Skeleton key={id} className='h-36 w-full rounded-xl' />
              ))}
            </div>
          ) : null}

          {!query.isLoading && items.length === 0 ? (
            <Empty className='border'>
              <EmptyHeader>
                <EmptyMedia variant='icon'>
                  <Database aria-hidden='true' />
                </EmptyMedia>
                <EmptyTitle>{t('No TypeSafe evaluations')}</EmptyTitle>
                <EmptyDescription>
                  {t(
                    'TypeSafe evaluations will appear here after requests run with a TypeSafe integration'
                  )}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : null}

          {items.length > 0 ? (
            <div className='space-y-3'>
              {items.map((item) => {
                const evaluationModels = [
                  ...new Set(
                    [item.before?.model, item.after?.model].filter(Boolean)
                  ),
                ]
                return (
                  <Card key={`${item.request_id}-${item.created_at}`} size='sm'>
                    <CardHeader className='gap-2'>
                      <CardTitle className='text-sm font-medium'>
                        {evaluationModels.length > 0
                          ? `${t('Evaluation model')}: ${evaluationModels.join(' / ')}`
                          : t('Evaluation model not recorded')}
                      </CardTitle>
                      <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
                        <span>{formatTimestampToDate(item.created_at)}</span>
                        <span>
                          {t('Evaluated model')}: {item.model_name || '-'}
                        </span>
                        {item.token_name ? (
                          <span>
                            {t('Token Name')}: {item.token_name}
                          </span>
                        ) : null}
                        {item.request_id ? (
                          <span className='font-mono'>
                            {t('Request ID')}: {item.request_id}
                          </span>
                        ) : null}
                      </div>
                    </CardHeader>
                    <CardContent>
                      <TypeSafeBeforeAfter
                        before={item.before}
                        after={item.after}
                        onViewDetails={(stage) =>
                          setDetail({ requestId: item.request_id, stage })
                        }
                      />
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          ) : null}

          {total > PAGE_SIZE ? (
            <div className='flex items-center justify-between gap-2 pt-1'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={!canPrev || query.isFetching}
                onClick={() => setPage((current) => Math.max(1, current - 1))}
              >
                {t('Previous')}
              </Button>
              <span className='text-muted-foreground text-sm'>
                {t('Page')} {page}
              </span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={!canNext || query.isFetching}
                onClick={() => setPage((current) => current + 1)}
              >
                {t('Next')}
              </Button>
            </div>
          ) : null}
        </div>
        {detail ? (
          <EvaluationDetailDialog
            key={`${detail.requestId}-${detail.stage}`}
            requestId={detail.requestId}
            stage={detail.stage}
            onClose={() => setDetail(null)}
          />
        ) : null}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
