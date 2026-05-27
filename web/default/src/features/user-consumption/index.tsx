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
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import {
  formatLogQuota,
  formatTimestampToDate,
  formatTokens,
  parseTimestampFromInput,
} from '@/lib/format'
import { getUserConsumption } from './api'
import { TokenStatCards, TokenConsumptionCharts } from './components'

const daySeconds = 24 * 60 * 60

function getDefaultStartTimestamp() {
  return Math.floor(Date.now() / 1000) - 30 * daySeconds
}

function getDefaultEndTimestamp() {
  return Math.floor(Date.now() / 1000)
}

function formatDatetimeInput(timestamp: number) {
  const date = new Date(timestamp * 1000)
  const timezoneOffset = date.getTimezoneOffset() * 60 * 1000
  return new Date(date.getTime() - timezoneOffset).toISOString().slice(0, 16)
}

export function UserConsumption() {
  const { t } = useTranslation()
  const [startInput, setStartInput] = useState(() =>
    formatDatetimeInput(getDefaultStartTimestamp())
  )
  const [endInput, setEndInput] = useState(() =>
    formatDatetimeInput(getDefaultEndTimestamp())
  )
  const [username, setUsername] = useState('')
  const [tokenName, setTokenName] = useState('')
  const [authIndex, setAuthIndex] = useState('')

  const filters = useMemo(
    () => ({
      p: 1,
      page_size: 100,
      start_timestamp: parseTimestampFromInput(startInput),
      end_timestamp: parseTimestampFromInput(endInput),
      username: username.trim() || undefined,
      token_name: tokenName.trim() || undefined,
      auth_index: authIndex.trim() || undefined,
    }),
    [authIndex, endInput, startInput, tokenName, username]
  )

  const query = useQuery({
    queryKey: ['cliproxy-user-consumption', filters],
    queryFn: () => getUserConsumption(filters),
  })

  const rows = query.data?.data?.items ?? []

  const tokenGroupedData = useMemo(() => {
    const tokenMap = new Map<number, {
      token_id: number
      token_name: string
      total_tokens: number
      prompt_tokens: number
      completion_tokens: number
      request_count: number
      quota: number
      users: Set<string>
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
        users: new Set(),
        last_called_at: 0,
      }

      existing.total_tokens += row.total_tokens
      existing.prompt_tokens += row.prompt_tokens
      existing.completion_tokens += row.completion_tokens
      existing.request_count += row.request_count
      existing.quota += row.quota
      if (row.username) existing.users.add(row.username)
      existing.last_called_at = Math.max(existing.last_called_at, row.last_called_at)

      tokenMap.set(row.token_id, existing)
    }

    return Array.from(tokenMap.values()).sort((a, b) => b.total_tokens - a.total_tokens)
  }, [rows])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('User Consumption')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' onClick={() => query.refetch()}>
          <RefreshCw className={query.isFetching ? 'animate-spin' : undefined} />
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
              <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-5'>
                <Input
                  type='datetime-local'
                  value={startInput}
                  onChange={(event) => setStartInput(event.target.value)}
                />
                <Input
                  type='datetime-local'
                  value={endInput}
                  onChange={(event) => setEndInput(event.target.value)}
                />
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
            </CardContent>
          </Card>

          <TokenStatCards data={rows} loading={query.isLoading} />

          <TokenConsumptionCharts data={rows} loading={query.isLoading} />

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
                          <div className='text-sm'>{row.users.size}</div>
                          <div className='text-muted-foreground text-xs'>
                            {Array.from(row.users).slice(0, 3).join(', ')}
                            {row.users.size > 3 && ` +${row.users.size - 3}`}
                          </div>
                        </TableCell>
                        <TableCell>{row.request_count}</TableCell>
                        <TableCell>{formatTokens(row.prompt_tokens)}</TableCell>
                        <TableCell>{formatTokens(row.completion_tokens)}</TableCell>
                        <TableCell>{formatTokens(row.total_tokens)}</TableCell>
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
