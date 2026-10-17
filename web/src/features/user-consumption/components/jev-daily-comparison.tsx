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
import { RefreshCw } from 'lucide-react'
import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DatePicker } from '@/components/date-picker'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import dayjs from '@/lib/dayjs'
import { formatTimestampToDate } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import { getJevDailyUsage } from '../api'
import { getJevDayRange } from '../lib/jev-daily'
import { JevAuthDifferenceTable } from './jev-auth-difference-table'
import { JevDifferenceSummary } from './jev-difference-summary'
import { JevAuthUsageTable, JevCohortTable } from './jev-usage-tables'

export function JevDailyComparison() {
  const { t } = useTranslation()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const switchId = useId()
  const modelId = useId()
  const searchId = useId()
  const [date, setDate] = useState(() => dayjs().subtract(1, 'day').toDate())
  const [comparisonDate, setComparisonDate] = useState(() =>
    dayjs().subtract(2, 'day').toDate()
  )
  const [compare, setCompare] = useState(true)
  const [search, setSearch] = useState('')
  const [selectedModel, setSelectedModel] = useState('all')
  const range = getJevDayRange(date)
  const comparisonRange = getJevDayRange(comparisonDate)
  const query = useQuery({
    queryKey: [
      'jev-daily-usage',
      userId,
      range.start_timestamp,
      range.end_timestamp,
      compare ? comparisonRange.start_timestamp : null,
      compare ? comparisonRange.end_timestamp : null,
    ],
    queryFn: async ({ signal }) =>
      Promise.all([
        getJevDailyUsage(range, signal),
        ...(compare && comparisonRange.start_timestamp !== range.start_timestamp
          ? [getJevDailyUsage(comparisonRange, signal)]
          : []),
      ]),
    retry: false,
    enabled: userId !== undefined,
    staleTime: 60_000,
  })
  const models = useMemo(
    () =>
      [
        ...new Set(
          (query.data ?? []).flatMap((report) =>
            report.rows.map((row) => row.model_name).filter(Boolean)
          )
        ),
      ].sort(),
    [query.data]
  )
  const model = models.includes(selectedModel) ? selectedModel : 'all'
  const reports = useMemo(
    () =>
      (query.data ?? []).map((report) => ({
        ...report,
        rows: report.rows.filter((row) => {
          const matchesSearch =
            `${row.username} ${row.auth_index} ${row.auth_name} ${row.provider}`
              .toLowerCase()
              .includes(search.trim().toLowerCase())
          return matchesSearch && (model === 'all' || row.model_name === model)
        }),
      })),
    [model, query.data, search]
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Jev daily comparison')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          <RefreshCw
            className={query.isFetching ? 'animate-spin' : undefined}
            aria-hidden='true'
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <Card>
            <CardHeader>
              <CardTitle>{t('Daily analysis filters')}</CardTitle>
              <CardDescription>
                {t(
                  'Compare recorded usage with and without Jev, then inspect each auth file.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='flex flex-wrap items-end gap-4'>
                <section aria-label={t('Analysis date')} className='space-y-2'>
                  <h3 className='text-sm font-medium'>{t('Analysis date')}</h3>
                  <DatePicker
                    selected={date}
                    onSelect={(value) => value && setDate(value)}
                  />
                </section>
                <div className='flex items-center gap-2 pb-2'>
                  <Switch
                    id={switchId}
                    checked={compare}
                    onCheckedChange={setCompare}
                  />
                  <Label htmlFor={switchId}>{t('Compare another day')}</Label>
                </div>
                {compare && (
                  <section
                    aria-label={t('Baseline date')}
                    className='space-y-2'
                  >
                    <h3 className='text-sm font-medium'>
                      {t('Baseline date')}
                    </h3>
                    <DatePicker
                      selected={comparisonDate}
                      onSelect={(value) => value && setComparisonDate(value)}
                    />
                  </section>
                )}
              </div>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Calendar day in browser timezone: {{timezone}}. End time is exclusive.',
                  { timezone: Intl.DateTimeFormat().resolvedOptions().timeZone }
                )}
              </p>
              <div className='grid gap-4 md:grid-cols-2'>
                <div className='space-y-2'>
                  <Label htmlFor={searchId}>{t('User or auth file')}</Label>
                  <Input
                    id={searchId}
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder={t(
                      'Filter by username, auth file, or provider'
                    )}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor={modelId}>{t('Model')}</Label>
                  <NativeSelect
                    id={modelId}
                    value={model}
                    onChange={(event) => setSelectedModel(event.target.value)}
                  >
                    <NativeSelectOption value='all'>
                      {t('All Models')}
                    </NativeSelectOption>
                    {models.map((name) => (
                      <NativeSelectOption key={name} value={name}>
                        {name}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </div>
              </div>
            </CardContent>
          </Card>
          <Alert>
            <AlertDescription className='space-y-2'>
              <p>
                {t(
                  'Applied decisions (including reuse) and successful before/after evaluations count as Jev used. Skipped, failed, unapplied, and unreadable records are separate. No Jev record does not prove it was disabled.'
                )}
              </p>
              <p>
                {t(
                  'Costs use consumption logs only. Main requests and Jev evaluation costs are counted separately; evaluations are linked only to a matching parent in the same day. Unlinked evaluations are listed separately.'
                )}
              </p>
              <p>
                {t(
                  'Auth attribution uses the current user binding and logged token name, not the actual upstream account or remaining subscription quota. Daily differences are observational, not proven savings.'
                )}
              </p>
              {model !== 'all' && (
                <p>
                  {t(
                    'Model filtering excludes standalone or unlinked Jev costs because their main model cannot be confirmed.'
                  )}
                </p>
              )}
            </AlertDescription>
          </Alert>
          {query.isPending && (
            <div
              role='status'
              aria-label={t('Loading...')}
              className='space-y-3'
            >
              <Skeleton className='h-48' />
              <Skeleton className='h-64' />
            </div>
          )}
          {query.isError && (
            <ErrorState
              title={t('Failed to load Jev daily usage')}
              description={query.error.message}
              onRetry={() => void query.refetch()}
            />
          )}
          {!query.isError &&
            compare &&
            reports.length === 1 &&
            !query.isFetching && (
              <Alert>
                <AlertDescription>
                  {t('Choose two different dates to calculate a difference.')}
                </AlertDescription>
              </Alert>
            )}
          {!query.isError && compare && reports.length === 2 && (
            <>
              {(range.end_timestamp > Date.now() / 1000 ||
                comparisonRange.end_timestamp > Date.now() / 1000) && (
                <Alert>
                  <AlertDescription>
                    {t(
                      'One selected day is incomplete. Its totals are provisional.'
                    )}
                  </AlertDescription>
                </Alert>
              )}
              <JevDifferenceSummary
                current={reports[0]}
                baseline={reports[1]}
              />
              <JevAuthDifferenceTable
                key={`${range.start_timestamp}-${comparisonRange.start_timestamp}-${model}-${search}`}
                current={reports[0].rows}
                baseline={reports[1].rows}
              />
            </>
          )}
          {!query.isError &&
            reports.map((report) => {
              const label = dayjs
                .unix(report.start_timestamp)
                .format('YYYY-MM-DD')
              return (
                <section
                  key={report.start_timestamp}
                  aria-label={label}
                  className='space-y-4'
                >
                  <p className='text-muted-foreground text-xs'>
                    {formatTimestampToDate(report.start_timestamp)} ~{' '}
                    {formatTimestampToDate(report.end_timestamp)}
                  </p>
                  <JevCohortTable
                    title={`${t('Jev usage cohorts')} · ${label}`}
                    rows={report.rows}
                  />
                  <JevAuthUsageTable
                    key={`${report.start_timestamp}-${model}-${search}`}
                    title={`${t('Auth file consumption')} · ${label}`}
                    rows={report.rows}
                  />
                </section>
              )
            })}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
