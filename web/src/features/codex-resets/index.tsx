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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import {
  ExternalLink,
  Loader2,
  RefreshCw,
  RotateCcw,
  Timer,
  Trash2,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { ErrorState } from '@/components/error-state'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useTheme } from '@/context/theme-provider'
import { useIsAdmin } from '@/hooks/use-admin'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'

import { deleteCodexReset, getCodexResets, syncCodexResets } from './api'
import type {
  CodexResetEvent,
  CodexResetsData,
  CodexResetsHeatmapPoint,
} from './types'

const QUERY_KEY = ['codex-resets'] as const

function formatDays(value: number | undefined): string {
  if (value == null || Number.isNaN(value)) return '-'
  if (value < 1) return `${Math.round(value * 24)}h`
  return `${value.toFixed(1)}d`
}

function formatRelative(ts: number | undefined, now = Date.now()): string {
  if (!ts) return '-'
  const diffMs = now - ts * 1000
  if (diffMs < 0) return formatTimestampToDate(ts)
  const minutes = Math.floor(diffMs / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

function utcDateKey(ts: number): string {
  if (!ts) return ''
  return new Date(ts * 1000).toISOString().slice(0, 10)
}

function Heatmap({
  points,
  eventsByDate,
}: {
  points: CodexResetsHeatmapPoint[]
  eventsByDate: Map<string, CodexResetEvent[]>
}) {
  const { t } = useTranslation()

  const weeks = useMemo(() => {
    const cols: CodexResetsHeatmapPoint[][] = []
    for (let i = 0; i < points.length; i += 7) {
      cols.push(points.slice(i, i + 7))
    }
    return cols
  }, [points])

  const levelClass = (level: number) => {
    if (level >= 3) return 'bg-emerald-600 dark:bg-emerald-500'
    if (level === 2) return 'bg-emerald-500/80 dark:bg-emerald-400/80'
    if (level === 1) return 'bg-emerald-400/70 dark:bg-emerald-300/70'
    return 'bg-muted'
  }

  const openDayAnnouncement = (day: CodexResetsHeatmapPoint) => {
    if (!day.count) {
      toast.message(t('No reset announcement on this day.'))
      return
    }
    const dayEvents = [...(eventsByDate.get(day.date) ?? [])].sort(
      (a, b) => b.announced_at - a.announced_at
    )
    const withUrl = dayEvents.find((event) => event.tweet_url?.trim())
    if (withUrl?.tweet_url) {
      window.open(withUrl.tweet_url, '_blank', 'noopener,noreferrer')
      return
    }
    // Fallback: scroll to the first matching row in the recent list.
    const target = dayEvents[0]
    if (target?.tweet_id) {
      const el = document.getElementById(`codex-reset-row-${target.tweet_id}`)
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
        el.classList.add('bg-muted/70')
        window.setTimeout(() => el.classList.remove('bg-muted/70'), 1600)
        return
      }
    }
    toast.message(t('No source link for this reset day.'))
  }

  return (
    <TooltipProvider delay={0}>
      <div className='overflow-x-auto'>
        <div className='flex min-w-max gap-1'>
          {weeks.map((week, weekIdx) => (
            <div key={weekIdx} className='flex flex-col gap-1'>
              {week.map((day) => {
                const clickable = day.count > 0
                return (
                  <Tooltip key={day.date}>
                    <TooltipTrigger
                      render={
                        <button
                          type='button'
                          aria-label={t('{{date}}: {{count}} reset(s)', {
                            date: day.date,
                            count: day.count,
                          })}
                          onClick={() => openDayAnnouncement(day)}
                          className={cn(
                            'size-3 rounded-[2px] transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1',
                            levelClass(day.level),
                            clickable
                              ? 'cursor-pointer hover:ring-2 hover:ring-emerald-500/50'
                              : 'cursor-default'
                          )}
                        />
                      }
                    />
                    <TooltipContent side='top' sideOffset={6}>
                      <div className='flex flex-col gap-0.5'>
                        <span className='font-medium tabular-nums'>
                          {day.date}
                        </span>
                        <span className='text-background/80'>
                          {t('{{count}} reset(s)', { count: day.count })}
                        </span>
                        {clickable ? (
                          <span className='text-background/70 text-xs'>
                            {t('Click to open source announcement')}
                          </span>
                        ) : null}
                      </div>
                    </TooltipContent>
                  </Tooltip>
                )
              })}
            </div>
          ))}
        </div>
      </div>
    </TooltipProvider>
  )
}

export function CodexResetsPage() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const queryClient = useQueryClient()
  const { resolvedTheme } = useTheme()
  const [deleteTarget, setDeleteTarget] = useState<CodexResetEvent | null>(null)

  const query = useQuery({
    queryKey: QUERY_KEY,
    queryFn: async () => {
      const res = await getCodexResets()
      if (!res.success || !res.data) {
        throw new Error(res.message || 'Failed to load Codex Resets')
      }
      return res.data
    },
    refetchInterval: 60_000,
  })

  const syncMutation = useMutation({
    mutationFn: async () => {
      const res = await syncCodexResets()
      if (!res.success) {
        throw new Error(res.message || 'Sync failed')
      }
      return res.data
    },
    onSuccess: (data) => {
      toast.success(
        t('Synced {{inserted}} new reset(s)', {
          inserted: data?.inserted ?? 0,
        })
      )
      void queryClient.invalidateQueries({ queryKey: QUERY_KEY })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Sync failed'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      const res = await deleteCodexReset(id)
      if (!res.success) {
        throw new Error(res.message || t('Delete failed'))
      }
      return id
    },
    onSuccess: (id) => {
      // 先本地摘掉，避免等待 refetch 期间仍显示旧行。
      queryClient.setQueryData<CodexResetsData>(QUERY_KEY, (prev) => {
        if (!prev) return prev
        return {
          ...prev,
          events: (prev.events ?? []).filter((event) => event.id !== id),
        }
      })
      toast.success(t('Deleted successfully'))
      setDeleteTarget(null)
      void queryClient.invalidateQueries({ queryKey: QUERY_KEY })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Delete failed'))
    },
  })

  const data = query.data
  const stats = data?.stats
  const intervals = data?.charts.intervals ?? []
  const heatmap = data?.charts.heatmap ?? []
  const events = data?.events ?? []
  const eventsByDate = useMemo(() => {
    const map = new Map<string, CodexResetEvent[]>()
    for (const event of events) {
      const key = utcDateKey(event.announced_at)
      if (!key) continue
      const list = map.get(key)
      if (list) {
        list.push(event)
      } else {
        map.set(key, [event])
      }
    }
    return map
  }, [events])
  const isDark = resolvedTheme === 'dark'

  const intervalSpec = useMemo(() => {
    if (intervals.length === 0) return null
    const axisLabelColor = isDark
      ? 'rgba(255,255,255,0.72)'
      : 'rgba(15,23,42,0.72)'
    const gridColor = isDark
      ? 'rgba(255,255,255,0.08)'
      : 'rgba(15,23,42,0.08)'
    return {
      type: 'bar' as const,
      theme: isDark ? 'dark' : 'light',
      background: 'transparent',
      data: [
        {
          id: 'intervals',
          values: intervals.map((item) => ({
            date: item.date,
            days: item.interval_days,
          })),
        },
      ],
      xField: 'date',
      yField: 'days',
      axes: [
        {
          orient: 'bottom',
          type: 'band',
          label: {
            autoRotate: true,
            style: { fill: axisLabelColor },
          },
          domainLine: { style: { stroke: gridColor } },
          tick: { style: { stroke: gridColor } },
        },
        {
          orient: 'left',
          type: 'linear',
          title: {
            text: t('Days'),
            style: { fill: axisLabelColor },
          },
          label: { style: { fill: axisLabelColor } },
          grid: { style: { stroke: gridColor } },
          domainLine: { style: { stroke: gridColor } },
        },
      ],
      tooltip: {
        mark: {
          content: [
            { key: t('Date'), value: (d: { date?: string }) => d?.date ?? '' },
            {
              key: t('Interval'),
              value: (d: { days?: number }) =>
                d?.days != null ? `${d.days}d` : '',
            },
          ],
        },
      },
      bar: {
        style: {
          fill: '#10b981',
          cornerRadius: 3,
        },
      },
    }
  }, [intervals, isDark, t])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Codex Resets')}</span>
          <Badge variant='outline' className='shrink-0'>
            codex-resets.com
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void query.refetch()}
            disabled={query.isFetching}
          >
            {query.isFetching ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <RefreshCw className='size-4' />
            )}
            {t('Refresh')}
          </Button>
          {isAdmin ? (
            <Button
              size='sm'
              onClick={() => syncMutation.mutate()}
              disabled={syncMutation.isPending}
            >
              {syncMutation.isPending ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <RotateCcw className='size-4' />
              )}
              {t('Sync now')}
            </Button>
          ) : null}
          <Button
            variant='ghost'
            size='sm'
            render={
              <a
                href='https://codex-resets.com/'
                target='_blank'
                rel='noreferrer'
              />
            }
          >
            <ExternalLink className='size-4' />
            {t('Source')}
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {query.isLoading ? (
          <div className='space-y-4'>
            <div className='grid gap-4 md:grid-cols-4'>
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className='h-28' />
              ))}
            </div>
            <Skeleton className='h-56' />
            <Skeleton className='h-72' />
          </div>
        ) : query.isError ? (
          <ErrorState
            title={t('Failed to load Codex Resets')}
            description={(query.error as Error)?.message}
            onRetry={() => void query.refetch()}
          />
        ) : (
          <div className='space-y-4'>
            <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
              <Card>
                <CardHeader className='pb-2'>
                  <CardDescription>{t('Time since last reset')}</CardDescription>
                  <CardTitle className='text-2xl tabular-nums'>
                    {formatRelative(stats?.last_reset_at)}
                  </CardTitle>
                </CardHeader>
                <CardContent className='text-muted-foreground text-xs'>
                  {stats?.last_reset_at
                    ? formatTimestampToDate(stats.last_reset_at)
                    : t('No resets yet')}
                </CardContent>
              </Card>
              <Card>
                <CardHeader className='pb-2'>
                  <CardDescription>{t('Total resets')}</CardDescription>
                  <CardTitle className='text-2xl tabular-nums'>
                    {stats?.total ?? 0}
                  </CardTitle>
                </CardHeader>
                <CardContent className='text-muted-foreground text-xs'>
                  {t('Tracked from public announcements')}
                </CardContent>
              </Card>
              <Card>
                <CardHeader className='pb-2'>
                  <CardDescription>{t('Average interval')}</CardDescription>
                  <CardTitle className='text-2xl tabular-nums'>
                    {formatDays(stats?.avg_interval_days)}
                  </CardTitle>
                </CardHeader>
                <CardContent className='text-muted-foreground text-xs'>
                  {t('Between consecutive resets')}
                </CardContent>
              </Card>
              <Card>
                <CardHeader className='pb-2'>
                  <CardDescription>{t('Longest wait')}</CardDescription>
                  <CardTitle className='text-2xl tabular-nums'>
                    {formatDays(stats?.longest_wait_days)}
                  </CardTitle>
                </CardHeader>
                <CardContent className='text-muted-foreground flex items-center gap-1 text-xs'>
                  <Timer className='size-3.5' />
                  {t('Shortest')}: {formatDays(stats?.shortest_wait_days)}
                </CardContent>
              </Card>
            </div>

            <div className='grid gap-4 xl:grid-cols-2'>
              <Card>
                <CardHeader>
                  <CardTitle>{t('Reset calendar')}</CardTitle>
                  <CardDescription>
                    {t('Last 26 weeks (UTC). Brighter cells mean more resets.')}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {heatmap.length === 0 ? (
                    <div className='text-muted-foreground text-sm'>
                      {t('No heatmap data yet. Sync to populate history.')}
                    </div>
                  ) : (
                    <Heatmap points={heatmap} eventsByDate={eventsByDate} />
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('Interval trend')}</CardTitle>
                  <CardDescription>
                    {t('Days waited before each reset')}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {intervalSpec ? (
                    <div className='bg-transparent h-64 w-full'>
                      <VChart
                        key={`interval-trend-${resolvedTheme}`}
                        spec={intervalSpec}
                        option={VCHART_OPTION}
                      />
                    </div>
                  ) : (
                    <div className='text-muted-foreground text-sm'>
                      {t('Need at least two resets to plot intervals.')}
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
                <div>
                  <CardTitle>{t('Recent resets')}</CardTitle>
                  <CardDescription>
                    {t('Latest announcements from @thsottiaux')}
                    {data?.sync.last_success_at
                      ? ` · ${t('Synced')} ${formatRelative(data.sync.last_success_at)}`
                      : ''}
                  </CardDescription>
                </div>
                {data?.sync.last_error ? (
                  <Badge variant='destructive' className='max-w-[240px] truncate'>
                    {data.sync.last_error}
                  </Badge>
                ) : null}
              </CardHeader>
              <CardContent>
                {events.length === 0 ? (
                  <div className='text-muted-foreground text-sm'>
                    {t(
                      'No local data yet. Admins can click Sync now to pull history.'
                    )}
                  </div>
                ) : (
                  <div className='overflow-x-auto'>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('When')}</TableHead>
                          <TableHead>{t('Announcement')}</TableHead>
                          <TableHead className='w-[100px]'>{t('Link')}</TableHead>
                          {isAdmin ? (
                            <TableHead className='w-[80px] text-right'>
                              {t('Actions')}
                            </TableHead>
                          ) : null}
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {events.slice(0, 30).map((event) => (
                          <TableRow
                            key={event.tweet_id}
                            id={`codex-reset-row-${event.tweet_id}`}
                            className='scroll-mt-24 transition-colors'
                          >
                            <TableCell className='whitespace-nowrap align-top tabular-nums'>
                              <div>{formatTimestampToDate(event.announced_at)}</div>
                              <div className='text-muted-foreground text-xs'>
                                {formatRelative(event.announced_at)}
                              </div>
                            </TableCell>
                            <TableCell className='max-w-[640px] whitespace-pre-wrap text-sm'>
                              {event.text}
                            </TableCell>
                            <TableCell className='align-top'>
                              {event.tweet_url ? (
                                <Button
                                  variant='ghost'
                                  size='sm'
                                  render={
                                    <a
                                      href={event.tweet_url}
                                      target='_blank'
                                      rel='noreferrer'
                                    />
                                  }
                                >
                                  <ExternalLink className='size-4' />
                                </Button>
                              ) : (
                                '-'
                              )}
                            </TableCell>
                            {isAdmin ? (
                              <TableCell className='align-top text-right'>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='sm'
                                  className='text-destructive hover:text-destructive'
                                  onClick={() => {
                                    if (!event.id) {
                                      toast.error(t('Invalid reset event id'))
                                      return
                                    }
                                    setDeleteTarget(event)
                                  }}
                                  disabled={
                                    deleteMutation.isPending &&
                                    deleteTarget?.id === event.id
                                  }
                                  aria-label={t('Delete')}
                                >
                                  {deleteMutation.isPending &&
                                  deleteTarget?.id === event.id ? (
                                    <Loader2 className='size-4 animate-spin' />
                                  ) : (
                                    <Trash2 className='size-4' />
                                  )}
                                </Button>
                              </TableCell>
                            ) : null}
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </SectionPageLayout.Content>

      <AlertDialog
        open={deleteTarget != null}
        onOpenChange={(open) => {
          if (!open && !deleteMutation.isPending) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete reset event')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Remove this reset from local history? It will stay removed even after later syncs from the upstream source.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deleteTarget?.text ? (
            <p className='text-muted-foreground line-clamp-3 text-sm'>
              {deleteTarget.text}
            </p>
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              type='button'
              variant='destructive'
              disabled={deleteMutation.isPending || !deleteTarget?.id}
              onClick={(event) => {
                // 阻止可能的默认关闭/提交，确保异步删除请求发出。
                event.preventDefault()
                event.stopPropagation()
                const id = deleteTarget?.id
                if (!id) {
                  toast.error(t('Invalid reset event id'))
                  return
                }
                deleteMutation.mutate(id)
              }}
            >
              {deleteMutation.isPending ? t('Deleting...') : t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SectionPageLayout>
  )
}
