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

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import dayjs from 'dayjs'
import { Eye, EyeOff, RefreshCw } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import {
  getCliproxyAuthFileBindings,
  refreshCliproxyAuthFileBindingUsage,
} from '../cliproxy-auth-files/api'
import type { CliproxyAuthFileBinding } from '../cliproxy-auth-files/types'
import { fetchAllXAIQuotaBindings } from './lib/xai-bindings'
import {
  buildXAIQuotaSummary,
  maskXAIAccountName,
  remainingProgressClass,
  type XAIQuotaProgress,
} from './lib/xai-usage'

const accountDisplayStorageKey = 'xai-quota-account-display'
const xaiBindingsQueryKey = ['cliproxy-auth-file-bindings', 'xai'] as const

function compactTimestamp(timestamp: number): string {
  if (timestamp <= 0) return '-'
  return dayjs.unix(timestamp).format('MM/DD HH:mm')
}

function QuotaProgressRow(props: {
  label: string
  progress: XAIQuotaProgress
  details?: ReactNode
}) {
  const { t } = useTranslation()

  return (
    <div className='space-y-1.5'>
      <div className='flex min-w-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm'>
        <span className='min-w-0 font-medium'>{props.label}</span>
        <div className='text-muted-foreground flex min-w-0 flex-wrap items-center justify-end gap-x-2 gap-y-1 text-xs tabular-nums'>
          <span className='text-foreground font-medium'>
            {t('Used {{percent}}%', {
              percent: props.progress.usedPercent,
            })}
          </span>
          {props.details}
        </div>
      </div>
      <Progress
        value={props.progress.remainingPercent}
        className={cn(
          '[&_[data-slot=progress-track]]:h-2',
          remainingProgressClass(props.progress.remainingPercent)
        )}
      />
    </div>
  )
}

function XAIQuotaCard(props: {
  binding: CliproxyAuthFileBinding
  showFullAccount: boolean
  refreshing: boolean
  onRefresh: () => void
}) {
  const { t } = useTranslation()
  const summary = buildXAIQuotaSummary(props.binding)
  const accountName =
    props.binding.auth_name ||
    props.binding.auth_file ||
    props.binding.auth_index
  const displayName = props.showFullAccount
    ? accountName
    : maskXAIAccountName(accountName)

  return (
    <Card className='from-muted/45 to-card gap-0 bg-gradient-to-b py-0 transition-shadow hover:shadow-sm'>
      <CardHeader className='border-b border-dashed py-4'>
        <CardTitle className='flex min-w-0 items-center gap-3'>
          <Badge
            variant='outline'
            className='bg-background h-8 shrink-0 px-3 text-sm font-semibold'
          >
            xAI
          </Badge>
          <span className='truncate' title={displayName}>
            {displayName}
          </span>
        </CardTitle>
      </CardHeader>

      <CardContent className='flex flex-1 flex-col gap-4 py-4'>
        <div className='text-sm'>
          <span className='text-muted-foreground mr-2'>{t('Plan')}</span>
          <span className='font-semibold'>{summary.planLabel}</span>
        </div>

        {summary.weekly.available ? (
          <QuotaProgressRow
            label={t('Weekly Limit')}
            progress={summary.weekly}
            details={
              <>
                <span>
                  {compactTimestamp(summary.weekly.periodStartAt)} ~{' '}
                  {compactTimestamp(summary.weekly.periodEndAt)}
                </span>
                <span>
                  {t('Reset {{time}}', {
                    time: compactTimestamp(summary.weekly.periodEndAt),
                  })}
                </span>
              </>
            }
          />
        ) : null}

        {summary.products.map((product) => (
          <QuotaProgressRow
            key={product.product}
            label={t('{{product}} Usage', { product: product.label })}
            progress={product}
          />
        ))}

        {summary.payAsYouGo.enabled ? (
          <div className='space-y-1.5'>
            <div className='flex flex-wrap items-center justify-between gap-2 text-sm'>
              <span className='font-medium'>{t('Pay-as-you-go')}</span>
              <span className='text-muted-foreground text-xs tabular-nums'>
                <span className='text-foreground font-medium'>
                  {summary.payAsYouGo.remainingPercent}%
                </span>{' '}
                {summary.payAsYouGo.remainingLabel} /{' '}
                {summary.payAsYouGo.limitLabel}
              </span>
            </div>
            <Progress
              value={summary.payAsYouGo.remainingPercent}
              className={cn(
                '[&_[data-slot=progress-track]]:h-2',
                remainingProgressClass(summary.payAsYouGo.remainingPercent)
              )}
            />
          </div>
        ) : (
          <div className='text-sm'>
            <span className='text-muted-foreground mr-2'>
              {t('Pay-as-you-go')}
            </span>
            <span className='font-medium'>{t('Not enabled')}</span>
          </div>
        )}

        {summary.monthly.enabled ? (
          <div className='space-y-1.5'>
            <div className='flex min-w-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm'>
              <span className='font-medium'>{t('Monthly Credits')}</span>
              <div className='text-muted-foreground flex min-w-0 flex-wrap items-center justify-end gap-x-2 gap-y-1 text-xs tabular-nums'>
                <span className='text-foreground font-medium'>
                  {summary.monthly.remainingPercent}%
                </span>
                <span>
                  {summary.monthly.remainingLabel} /{' '}
                  {summary.monthly.limitLabel}
                </span>
                <span>
                  {compactTimestamp(summary.monthly.billingPeriodEndAt)}
                </span>
              </div>
            </div>
            <Progress
              value={summary.monthly.remainingPercent}
              className={cn(
                '[&_[data-slot=progress-track]]:h-2',
                remainingProgressClass(summary.monthly.remainingPercent)
              )}
            />
          </div>
        ) : null}

        {props.binding.last_error ? (
          <Alert className='mt-auto py-2'>
            <AlertDescription className='text-xs'>
              {t('Some quota data could not be refreshed.')}{' '}
              {props.binding.last_error}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>

      <CardFooter className='justify-end bg-transparent px-4 py-3'>
        <Button
          variant='outline'
          size='lg'
          className='rounded-full px-4'
          onClick={props.onRefresh}
          disabled={props.refreshing}
        >
          <RefreshCw
            className={props.refreshing ? 'animate-spin' : undefined}
          />
          {t('Refresh Quota')}
        </Button>
      </CardFooter>
    </Card>
  )
}

function XAIQuotaSkeleton() {
  return (
    <Card className='gap-0 py-0'>
      <CardHeader className='border-b border-dashed py-4'>
        <div className='flex items-center gap-3'>
          <Skeleton className='h-8 w-14 rounded-full' />
          <Skeleton className='h-5 w-2/3' />
        </div>
      </CardHeader>
      <CardContent className='space-y-5 py-4'>
        <Skeleton className='h-5 w-32' />
        {Array.from({ length: 3 }, (_, index) => (
          <div key={index} className='space-y-2'>
            <Skeleton className='h-4 w-full' />
            <Skeleton className='h-2 w-full rounded-full' />
          </div>
        ))}
        <div className='flex justify-end'>
          <Skeleton className='h-9 w-28 rounded-full' />
        </div>
      </CardContent>
    </Card>
  )
}

export function XAIQuotaPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [showFullAccount, setShowFullAccount] = useState(() => {
    if (typeof window === 'undefined') return false
    return window.localStorage.getItem(accountDisplayStorageKey) === 'full'
  })
  const [refreshingIds, setRefreshingIds] = useState<Set<number>>(new Set())
  const query = useQuery({
    queryKey: xaiBindingsQueryKey,
    queryFn: () => fetchAllXAIQuotaBindings(getCliproxyAuthFileBindings),
  })

  const responseError =
    query.data && !query.data.success
      ? query.data.message || t('Failed to fetch xAI quota bindings')
      : null
  const bindings = query.data?.data?.items ?? []
  const total = query.data?.data?.total ?? 0
  const queryErrorMessage =
    responseError ||
    (query.error instanceof Error
      ? query.error.message
      : t('Failed to fetch xAI quota bindings'))

  const toggleAccountDisplay = () => {
    setShowFullAccount((current) => {
      const next = !current
      window.localStorage.setItem(
        accountDisplayStorageKey,
        next ? 'full' : 'masked'
      )
      return next
    })
  }

  const refreshBinding = async (binding: CliproxyAuthFileBinding) => {
    setRefreshingIds((current) => new Set(current).add(binding.id))
    try {
      const response = await refreshCliproxyAuthFileBindingUsage(binding.id)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: xaiBindingsQueryKey }),
        queryClient.invalidateQueries({
          queryKey: ['cliproxy-auth-file-bindings'],
        }),
      ])
      if (!response.success) {
        toast.error(response.message || t('Refresh failed'))
        return
      }
      if (response.data?.last_error) {
        toast.warning(t('Some quota data could not be refreshed.'))
      } else {
        toast.success(t('Quota refreshed successfully'))
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Refresh failed'))
    } finally {
      setRefreshingIds((current) => {
        const next = new Set(current)
        next.delete(binding.id)
        return next
      })
    }
  }

  let content: ReactNode
  if (query.isLoading) {
    content = (
      <div className='grid gap-4 md:grid-cols-2 2xl:grid-cols-3'>
        {Array.from({ length: 4 }, (_, index) => (
          <XAIQuotaSkeleton key={index} />
        ))}
      </div>
    )
  } else if (query.isError || responseError) {
    content = (
      <Alert variant='destructive'>
        <AlertDescription className='flex flex-wrap items-center justify-between gap-3'>
          <span>{queryErrorMessage}</span>
          <Button variant='outline' size='sm' onClick={() => query.refetch()}>
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  } else if (bindings.length === 0) {
    content = (
      <Card>
        <CardContent className='flex min-h-48 flex-col items-center justify-center gap-4 text-center'>
          <p className='text-muted-foreground'>
            {t('No xAI auth files are bound yet.')}
          </p>
          <Button variant='outline' render={<Link to='/cliproxy-auth-files' />}>
            {t('Manage auth files')}
          </Button>
        </CardContent>
      </Card>
    )
  } else {
    content = (
      <div className='grid gap-4 md:grid-cols-2 2xl:grid-cols-3'>
        {bindings.map((binding) => (
          <XAIQuotaCard
            key={binding.id}
            binding={binding}
            showFullAccount={showFullAccount}
            refreshing={refreshingIds.has(binding.id)}
            onRefresh={() => refreshBinding(binding)}
          />
        ))}
      </div>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex items-center gap-2'>
          <span>{t('xAI Quota')}</span>
          <Badge variant='secondary'>{total}</Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='lg'
          aria-pressed={showFullAccount}
          onClick={toggleAccountDisplay}
        >
          {showFullAccount ? <EyeOff /> : <Eye />}
          {showFullAccount ? t('Mask accounts') : t('Show full accounts')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
