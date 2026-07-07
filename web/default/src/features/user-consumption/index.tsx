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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TIME_RANGE_PRESETS } from '@/features/dashboard/constants'
import {
  formatLogQuota,
  formatTimestampToDate,
  formatTokenDetails,
  formatTokens,
} from '@/lib/format'
import { getRollingDateRange } from '@/lib/time'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getUserConsumption } from './api'
import { TokenStatCards, TokenConsumptionCharts } from './components'
import type { UserConsumptionSummary } from './types'

const defaultRangeDays = 29

function toUnixSeconds(date: Date | undefined): number | undefined {
  return date ? Math.floor(date.getTime() / 1000) : undefined
}

function TokenAmount({ value }: { value: number }) {
  const tokenValue = Number(value || 0)
  return (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger render={<span className='cursor-default font-mono' />}>
          {formatTokens(tokenValue)}
        </TooltipTrigger>
        <TooltipContent>{formatTokenDetails(tokenValue)}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function UserSummaryCell({
  users,
}: {
  users: Array<Pick<UserConsumptionSummary, 'username' | 'remark'>>
}) {
  const { t } = useTranslation()
  const visibleUsers = users.slice(0, 3)

  return (
    <div className='space-y-1'>
      <div className='text-sm'>{users.length}</div>
      <TooltipProvider delay={200}>
        <Tooltip>
          <TooltipTrigger
            render={<div className='max-w-[220px] cursor-default' />}
          >
            <div className='text-muted-foreground space-y-0.5 text-xs'>
              {visibleUsers.map((user) => (
                <div key={user.username} className='min-w-0'>
                  <div className='truncate'>{user.username}</div>
                  {user.remark ? (
                    <div className='text-muted-foreground/70 truncate'>
                      {user.remark}
                    </div>
                  ) : null}
                </div>
              ))}
              {users.length > 3 ? <div>+{users.length - 3}</div> : null}
            </div>
          </TooltipTrigger>
          <TooltipContent
            side='top'
            align='start'
            className='max-w-xs whitespace-pre-wrap break-words'
          >
            <div className='space-y-2'>
              {users.map((user) => (
                <div key={user.username}>
                  <div>{user.username}</div>
                  {user.remark ? (
                    <div className='text-muted-foreground text-xs'>
                      {t('Remark')}: {user.remark}
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  )
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
      page_size: 100,
      start_timestamp: toUnixSeconds(timeRange.start),
      end_timestamp: toUnixSeconds(timeRange.end),
      username: username.trim() || undefined,
      token_name: tokenName.trim() || undefined,
      auth_index: authIndex.trim() || undefined,
    }),
    [authIndex, timeRange.end, timeRange.start, tokenName, username]
  )

  const rankFilters = useMemo(
    () => ({
      ...filters,
      p: 1,
      page_size: 500,
      sort_by: 'total_tokens',
      sort_order: 'desc',
    }),
    [filters]
  )

  const query = useQuery({
    queryKey: ['cliproxy-user-consumption', filters],
    queryFn: () => getUserConsumption(filters),
  })

  const rankQuery = useQuery({
    queryKey: ['cliproxy-user-consumption-rank', rankFilters],
    queryFn: () => getUserConsumption(rankFilters),
  })

  const rows = useMemo(
    () => query.data?.data?.items ?? [],
    [query.data?.data?.items]
  )

  const rankRows = useMemo(
    () => rankQuery.data?.data?.items ?? [],
    [rankQuery.data?.data?.items]
  )

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

  const tokenGroupedData = useMemo(() => {
    const tokenMap = new Map<number, {
      token_id: number
      token_name: string
      total_tokens: number
      prompt_tokens: number
      completion_tokens: number
      request_count: number
      quota: number
      users: Map<string, Pick<UserConsumptionSummary, 'username' | 'remark'>>
      last_called_at: number
    }>()

    for (const row of rows) {
      const existing = tokenMap.get(row.token_id) || {
        token_id: row.token_id,
        token_name: row.token_name,
        total_tokens: 0,
        prompt_tokens: 0,
        completion_tokens: 0,
        request_count: 0,
        quota: 0,
        users: new Map(),
        last_called_at: 0,
      }

      existing.total_tokens += row.total_tokens
      existing.prompt_tokens += row.prompt_tokens
      existing.completion_tokens += row.completion_tokens
      existing.request_count += row.request_count
      existing.quota += row.quota
      if (row.username && !existing.users.has(row.username)) {
        existing.users.set(row.username, {
          username: row.username,
          remark: row.remark || '',
        })
      }
      existing.last_called_at = Math.max(existing.last_called_at, row.last_called_at)

      tokenMap.set(row.token_id, existing)
    }

    return [...tokenMap.values()].sort((a, b) => b.total_tokens - a.total_tokens)
  }, [rows])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('User Consumption')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          onClick={() => {
            void query.refetch()
            void rankQuery.refetch()
          }}
        >
          <RefreshCw
            className={
              query.isFetching || rankQuery.isFetching
                ? 'animate-spin'
                : undefined
            }
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
                {t('Analyze token-level user consumption by time, user, token, and auth file.')}
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

          <TokenStatCards data={rows} loading={query.isLoading} />

          <TokenConsumptionCharts
            data={rankRows}
            loading={rankQuery.isLoading}
          />

          <Card>
            <CardHeader>
              <CardTitle>{t('Token Consumption Details')}</CardTitle>
              <CardDescription>
                {t('Aggregated by token with user breakdown.')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {query.data && !query.data.success ? (
                <Alert variant='destructive' className='mb-4'>
                  <AlertDescription>
                    {query.data.message || t('Failed to fetch user consumption')}
                  </AlertDescription>
                </Alert>
              ) : null}

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Token')}</TableHead>
                    <TableHead>{t('Users')}</TableHead>
                    <TableHead>{t('Requests')}</TableHead>
                    <TableHead>{t('Prompt Tokens')}</TableHead>
                    <TableHead>{t('Completion Tokens')}</TableHead>
                    <TableHead>{t('Total Tokens')}</TableHead>
                    <TableHead>{t('Quota')}</TableHead>
                    <TableHead>{t('Last Called')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tokenGroupedData.length > 0 ? (
                    tokenGroupedData.map((row) => (
                      <TableRow key={row.token_id}>
                        <TableCell>
                          <div className='font-medium'>{row.token_name || '-'}</div>
                          <div className='text-muted-foreground text-xs'>ID: {row.token_id}</div>
                        </TableCell>
                        <TableCell>
                          <UserSummaryCell users={[...row.users.values()]} />
                        </TableCell>
                        <TableCell>{row.request_count}</TableCell>
                        <TableCell><TokenAmount value={row.prompt_tokens} /></TableCell>
                        <TableCell><TokenAmount value={row.completion_tokens} /></TableCell>
                        <TableCell><TokenAmount value={row.total_tokens} /></TableCell>
                        <TableCell>{formatLogQuota(row.quota)}</TableCell>
                        <TableCell>{formatTimestampToDate(row.last_called_at)}</TableCell>
                      </TableRow>
                    ))
                  ) : (
                    <TableRow>
                      <TableCell colSpan={8} className='text-muted-foreground py-8 text-center'>
                        {query.isLoading ? t('Loading...') : t('No consumption data found')}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
