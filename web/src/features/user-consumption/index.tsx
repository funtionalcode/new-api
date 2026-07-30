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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CalendarRange, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { DateTimePicker } from '@/components/datetime-picker'
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
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TIME_RANGE_PRESETS } from '@/features/dashboard/constants'
import { formatTimestampToDate } from '@/lib/format'
import { getRollingDateRange } from '@/lib/time'
import { getUserConsumption } from './api'
import {
  TokenStatCards,
  TokenConsumptionCharts,
  TokenQuotaCharts,
  UserQuotaCharts,
} from './components'

const defaultRangeDays = 29

function toUnixSeconds(date: Date | undefined): number | undefined {
  return date ? Math.floor(date.getTime() / 1000) : undefined
}

export function UserConsumption() {
  const { t } = useTranslation()
  const [selectedRange, setSelectedRange] = useState<number | null>(
    defaultRangeDays
  )
  const [timeRange, setTimeRange] = useState<{
    start?: Date
    end?: Date
  }>(() => getRollingDateRange(defaultRangeDays))
  const [username, setUsername] = useState('')
  const [tokenName, setTokenName] = useState('')
  const [authIndex, setAuthIndex] = useState('')

  const filters = useMemo(
    () => ({
      p: 1,
      page_size: 500,
      start_timestamp: toUnixSeconds(timeRange.start),
      end_timestamp: toUnixSeconds(timeRange.end),
      username: username.trim() || undefined,
      token_name: tokenName.trim() || undefined,
      auth_index: authIndex.trim() || undefined,
      sort_by: 'total_tokens',
      sort_order: 'desc',
    }),
    [authIndex, timeRange.end, timeRange.start, tokenName, username]
  )

  const query = useQuery({
    queryKey: ['cliproxy-user-consumption', filters],
    queryFn: () => getUserConsumption(filters),
  })

  const rows = useMemo(
    () => query.data?.data?.items ?? [],
    [query.data?.data?.items]
  )

  const timeRangeKey = `${filters.start_timestamp ?? 0}-${filters.end_timestamp ?? 0}`

  const timeRangeLabel = useMemo(() => {
    return `${formatTimestampToDate(toUnixSeconds(timeRange.start))} ~ ${formatTimestampToDate(toUnixSeconds(timeRange.end))}`
  }, [timeRange.end, timeRange.start])

  const handleRangeChange = (days: number) => {
    setTimeRange(getRollingDateRange(days))
    setSelectedRange(days)
  }

  const handleStartChange = (date: Date | undefined) => {
    setTimeRange((prev) => ({ ...prev, start: date }))
    setSelectedRange(null)
  }

  const handleEndChange = (date: Date | undefined) => {
    setTimeRange((prev) => ({ ...prev, end: date }))
    setSelectedRange(null)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('User Consumption')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          onClick={() => {
            void query.refetch()
          }}
        >
          <RefreshCw
            className={query.isFetching ? 'animate-spin' : undefined}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <Card>
            <CardHeader>
              <CardTitle>{t('Consumption Filters')}</CardTitle>
              <CardDescription>
                {t(
                  'Analyze token-level user consumption by time, user, token, and auth file.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className='space-y-3'>
                <div className='flex flex-col gap-2 xl:flex-row xl:items-center xl:justify-between'>
                  <div className='text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs'>
                    <CalendarRange className='size-3.5 shrink-0' />
                    <span>{t('Date Range')}:</span>
                    <span className='truncate font-mono tabular-nums'>
                      {timeRangeLabel}
                    </span>
                  </div>

                  <div className='border-border/60 bg-muted/20 flex max-w-full flex-wrap items-center gap-2 rounded-md border px-2 py-1'>
                    <CalendarRange className='text-muted-foreground size-4 shrink-0' />
                    <DateTimePicker
                      value={timeRange.start}
                      onChange={handleStartChange}
                      placeholder={t('Select start time')}
                      className='w-[280px]'
                    />
                    <DateTimePicker
                      value={timeRange.end}
                      onChange={handleEndChange}
                      placeholder={t('Select end time')}
                      className='w-[280px]'
                    />
                  </div>
                </div>

                <div className='flex flex-wrap items-center gap-2'>
                  <Tabs
                    value={selectedRange == null ? '' : String(selectedRange)}
                    onValueChange={(value) => handleRangeChange(Number(value))}
                    className='shrink-0'
                  >
                    <TabsList>
                      {TIME_RANGE_PRESETS.map((preset) => (
                        <TabsTrigger
                          key={preset.days}
                          value={String(preset.days)}
                          className='px-2.5 text-xs'
                        >
                          {t(preset.label)}
                        </TabsTrigger>
                      ))}
                    </TabsList>
                  </Tabs>
                </div>

                <div className='grid gap-3 md:grid-cols-3'>
                  <Input
                    value={username}
                    placeholder={t('Filter by username')}
                    onChange={(event) => setUsername(event.target.value)}
                  />
                  <Input
                    value={tokenName}
                    placeholder={t('Filter by token name')}
                    onChange={(event) => setTokenName(event.target.value)}
                  />
                  <Input
                    value={authIndex}
                    placeholder={t('Filter by auth index')}
                    onChange={(event) => setAuthIndex(event.target.value)}
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          {query.data && !query.data.success ? (
            <Alert variant='destructive'>
              <AlertDescription>
                {query.data.message || t('Failed to fetch user consumption')}
              </AlertDescription>
            </Alert>
          ) : null}

          <TokenStatCards data={rows} loading={query.isLoading} />

          <div className='grid gap-4 xl:grid-cols-2'>
            <TokenConsumptionCharts
              data={rows}
              loading={query.isLoading}
              renderKey={timeRangeKey}
            />
            <TokenQuotaCharts
              data={rows}
              loading={query.isLoading}
              renderKey={timeRangeKey}
            />
            <UserQuotaCharts
              data={rows}
              loading={query.isLoading}
              renderKey={timeRangeKey}
            />
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
