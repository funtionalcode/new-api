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
import { DatabaseBackup, RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toIntlLocale } from '@/i18n/languages'
import { formatTimestampRelative, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  getCurrentDBBackupTask,
  getSystemTask,
  listSystemTasks,
  triggerDBBackup,
} from '../api'
import type {
  DBBackupArtifact,
  DBBackupResult,
  DBBackupTask,
  SystemTaskStatus,
} from '../types'

export const DB_BACKUP_HISTORY_QUERY_KEY = [
  'system-settings',
  'db-backup-history',
] as const

const HISTORY_LIMIT = 30
const ACTIVE_POLL_INTERVAL_MS = 4000

const STATUS_VARIANT: Record<SystemTaskStatus, 'secondary' | 'destructive'> = {
  pending: 'secondary',
  running: 'secondary',
  succeeded: 'secondary',
  failed: 'destructive',
}

const STATUS_CLASS_NAME: Record<SystemTaskStatus, string> = {
  pending:
    'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
  running:
    'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300',
  succeeded:
    'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
  failed: '',
}

const STATUS_DOT_CLASS_NAME: Record<SystemTaskStatus, string> = {
  pending: 'bg-amber-500',
  running: 'bg-sky-500',
  succeeded: 'bg-emerald-500',
  failed: 'bg-destructive',
}

function isActiveDBBackupTask(task: DBBackupTask | null | undefined) {
  return task?.status === 'pending' || task?.status === 'running'
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function asDBBackupResult(value: unknown): DBBackupResult | null {
  const record = asRecord(value)
  if (!record) return null
  return {
    artifacts: Array.isArray(record.artifacts)
      ? (record.artifacts as DBBackupArtifact[])
      : undefined,
    duration_ms:
      typeof record.duration_ms === 'number' ? record.duration_ms : undefined,
    host: typeof record.host === 'string' ? record.host : undefined,
    log_path: typeof record.log_path === 'string' ? record.log_path : undefined,
    log_dir: typeof record.log_dir === 'string' ? record.log_dir : undefined,
    log_excerpt:
      typeof record.log_excerpt === 'string' ? record.log_excerpt : undefined,
  }
}

function formatBytes(bytes: number | undefined): string {
  if (bytes == null || Number.isNaN(bytes)) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = -1
  do {
    value /= 1024
    unit += 1
  } while (value >= 1024 && unit < units.length - 1)
  return `${value.toFixed(1)} ${units[unit]}`
}

function taskHasDetail(task: DBBackupTask): boolean {
  if (task.error) return true
  if (task.result == null) return false
  if (typeof task.result === 'string') return task.result.trim() !== ''
  if (typeof task.result === 'object') return Object.keys(task.result).length > 0
  return true
}

function triggerSourceLabel(
  task: DBBackupTask,
  t: (key: string) => string
): string {
  const triggeredBy = task.payload?.triggered_by?.trim()
  if (!triggeredBy) return '-'
  if (triggeredBy === 'scheduler') return t('Scheduled')
  return triggeredBy
}

type DetailDialogProps = {
  task: DBBackupTask | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function DBBackupDetailDialog({ task, open, onOpenChange }: DetailDialogProps) {
  const { t, i18n } = useTranslation()
  const backupResult = useMemo(
    () => (task ? asDBBackupResult(task.result) : null),
    [task]
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Backup detail')}</DialogTitle>
          <DialogDescription>
            {task
              ? `${t('Database backup')} · ${task.task_id}`
              : t('No backup selected.')}
          </DialogDescription>
        </DialogHeader>
        {task ? (
          <div className='space-y-4'>
            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>{t('Status')}</div>
                <div className='text-sm font-medium'>{t(task.status)}</div>
              </div>
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>{t('Trigger source')}</div>
                <div className='text-sm'>{triggerSourceLabel(task, t)}</div>
              </div>
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>{t('Executor')}</div>
                <div className='font-mono text-xs break-all'>
                  {task.locked_by || '-'}
                </div>
              </div>
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>{t('Updated')}</div>
                <div className='text-sm'>
                  {formatTimestampToDate(task.updated_at)}
                  <span className='text-muted-foreground ml-2 text-xs'>
                    (
                    {formatTimestampRelative(
                      task.updated_at,
                      'seconds',
                      toIntlLocale(i18n.language)
                    )}
                    )
                  </span>
                </div>
              </div>
              {backupResult?.host ? (
                <div className='space-y-1'>
                  <div className='text-muted-foreground text-xs'>{t('Host')}</div>
                  <div className='font-mono text-xs break-all'>
                    {backupResult.host}
                  </div>
                </div>
              ) : null}
              {backupResult?.duration_ms != null ? (
                <div className='space-y-1'>
                  <div className='text-muted-foreground text-xs'>
                    {t('Duration')}
                  </div>
                  <div className='text-sm tabular-nums'>
                    {(backupResult.duration_ms / 1000).toFixed(1)}s
                  </div>
                </div>
              ) : null}
            </div>

            {task.error ? (
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>{t('Error')}</div>
                <pre className='bg-destructive/5 text-destructive max-h-40 overflow-auto rounded-md border p-3 text-xs whitespace-pre-wrap'>
                  {task.error}
                </pre>
              </div>
            ) : null}

            {backupResult ? (
              <>
                {(backupResult.log_dir || backupResult.log_path) && (
                  <div className='grid gap-3 sm:grid-cols-2'>
                    {backupResult.log_dir ? (
                      <div className='space-y-1'>
                        <div className='text-muted-foreground text-xs'>
                          {t('Log directory')}
                        </div>
                        <div className='font-mono text-xs break-all'>
                          {backupResult.log_dir}
                        </div>
                      </div>
                    ) : null}
                    {backupResult.log_path ? (
                      <div className='space-y-1'>
                        <div className='text-muted-foreground text-xs'>
                          {t('Log file')}
                        </div>
                        <div className='font-mono text-xs break-all'>
                          {backupResult.log_path}
                        </div>
                      </div>
                    ) : null}
                  </div>
                )}

                {backupResult.artifacts && backupResult.artifacts.length > 0 ? (
                  <div className='space-y-2'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Artifacts')}
                    </div>
                    <div className='overflow-x-auto rounded-md border'>
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead className='text-xs'>{t('Type')}</TableHead>
                            <TableHead className='text-xs'>{t('File')}</TableHead>
                            <TableHead className='text-xs'>{t('Size')}</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {backupResult.artifacts.map((artifact, index) => (
                            <TableRow key={`${artifact.file}-${index}`}>
                              <TableCell className='text-xs'>
                                {artifact.type}
                                {artifact.database
                                  ? ` · ${artifact.database}`
                                  : ''}
                              </TableCell>
                              <TableCell className='max-w-[280px] truncate font-mono text-[11px]'>
                                {artifact.file}
                              </TableCell>
                              <TableCell className='text-xs tabular-nums'>
                                {formatBytes(artifact.size_bytes)}
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  </div>
                ) : null}

                {backupResult.log_excerpt ? (
                  <div className='space-y-1'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Backup log')}
                    </div>
                    <pre className='bg-muted/40 max-h-72 overflow-auto rounded-md border p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap'>
                      {backupResult.log_excerpt}
                    </pre>
                  </div>
                ) : (
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'No log excerpt reported. Ensure the host backup script writes to the configured log directory and reports log_excerpt.'
                    )}
                  </p>
                )}
              </>
            ) : !task.error ? (
              <p className='text-muted-foreground text-sm'>
                {t('No additional detail for this task.')}
              </p>
            ) : null}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

export function DBBackupHistoryPanel() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const [detailTask, setDetailTask] = useState<DBBackupTask | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const [showTriggerConfirm, setShowTriggerConfirm] = useState(false)
  const [isStarting, setIsStarting] = useState(false)
  const [trackedTask, setTrackedTask] = useState<DBBackupTask | null>(null)

  const historyQuery = useQuery({
    queryKey: DB_BACKUP_HISTORY_QUERY_KEY,
    queryFn: async () => {
      const res = await listSystemTasks(HISTORY_LIMIT, 'db_backup')
      if (!res.success || !Array.isArray(res.data)) {
        throw new Error(res.message || t('Failed to load backup records.'))
      }
      return res.data as DBBackupTask[]
    },
    staleTime: 15 * 1000,
    retry: false,
    refetchInterval: (query) =>
      query.state.data?.some((task) => isActiveDBBackupTask(task))
        ? ACTIVE_POLL_INTERVAL_MS
        : false,
  })

  useEffect(() => {
    let cancelled = false
    async function loadCurrent() {
      try {
        const res = await getCurrentDBBackupTask()
        if (!cancelled && res.success && res.data) {
          setTrackedTask(res.data)
        }
      } catch {
        /* ignore */
      }
    }
    void loadCurrent()
    return () => {
      cancelled = true
    }
  }, [])

  const active = isActiveDBBackupTask(trackedTask)
  const taskId = trackedTask?.task_id

  useEffect(() => {
    if (!taskId || !active) return
    let cancelled = false
    const interval = window.setInterval(async () => {
      try {
        const res = await getSystemTask(taskId)
        if (cancelled || !res.success || !res.data) return
        const nextTask = res.data as DBBackupTask
        setTrackedTask(nextTask)
        if (!isActiveDBBackupTask(nextTask)) {
          void queryClient.invalidateQueries({
            queryKey: DB_BACKUP_HISTORY_QUERY_KEY,
          })
          void queryClient.invalidateQueries({
            queryKey: ['system-info', 'system-tasks'],
          })
          if (nextTask.status === 'succeeded') {
            toast.success(t('Database backup completed.'))
          } else if (nextTask.status === 'failed') {
            toast.error(nextTask.error || t('Database backup failed.'))
          }
        }
      } catch {
        /* keep polling */
      }
    }, 2000)
    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [active, queryClient, t, taskId])

  const tasks = historyQuery.data ?? []
  const refreshing = historyQuery.isFetching && !historyQuery.isLoading

  const handleTrigger = async () => {
    setIsStarting(true)
    try {
      const res = await triggerDBBackup()
      if (!res.success || !res.data) {
        throw new Error(res.message || t('Failed to start database backup.'))
      }
      setTrackedTask(res.data)
      setShowTriggerConfirm(false)
      void queryClient.invalidateQueries({
        queryKey: DB_BACKUP_HISTORY_QUERY_KEY,
      })
      void queryClient.invalidateQueries({
        queryKey: ['system-info', 'system-tasks'],
      })
      toast.success(
        res.message
          ? t('Database backup task is already running.')
          : t('Database backup task started.')
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to start database backup.')
      )
    } finally {
      setIsStarting(false)
    }
  }

  return (
    <div className='space-y-3'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0 space-y-1'>
          <h4 className='text-sm font-semibold'>{t('Backup records')}</h4>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Recent database backup runs, including status, artifacts, and host logs.'
            )}
          </p>
        </div>
        <div className='flex shrink-0 flex-wrap items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void historyQuery.refetch()}
            disabled={historyQuery.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn('size-3.5', refreshing && 'animate-spin')}
              aria-hidden='true'
            />
            {refreshing ? t('Refreshing...') : t('Refresh')}
          </Button>
          <Button
            type='button'
            size='sm'
            disabled={isStarting || active}
            onClick={() => setShowTriggerConfirm(true)}
          >
            <DatabaseBackup
              data-icon='inline-start'
              className='size-3.5'
              aria-hidden='true'
            />
            {isStarting || active
              ? t('Backup running...')
              : t('Trigger database backup')}
          </Button>
        </div>
      </div>

      {historyQuery.isLoading ? (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      ) : historyQuery.isError ? (
        <p className='text-destructive text-sm'>
          {historyQuery.error instanceof Error
            ? historyQuery.error.message
            : t('Failed to load backup records.')}
        </p>
      ) : tasks.length === 0 ? (
        <div className='text-muted-foreground rounded-md border border-dashed px-4 py-8 text-center text-sm'>
          {t('No backup records yet.')}
        </div>
      ) : (
        <div className='overflow-x-auto rounded-md border'>
          <Table className='min-w-[760px]'>
            <TableHeader>
              <TableRow className='bg-muted/40 hover:bg-muted/40'>
                <TableHead className='h-9 w-[150px] px-4 text-xs'>
                  {t('Status')}
                </TableHead>
                <TableHead className='h-9 w-[140px] text-xs'>
                  {t('Trigger source')}
                </TableHead>
                <TableHead className='h-9 min-w-[180px] text-xs'>
                  {t('Executor')}
                </TableHead>
                <TableHead className='h-9 w-[120px] text-xs'>
                  {t('Duration')}
                </TableHead>
                <TableHead className='h-9 w-[170px] text-xs'>
                  {t('Updated')}
                </TableHead>
                <TableHead className='h-9 w-[100px] pr-4 text-xs'>
                  {t('Detail')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tasks.map((task) => {
                const result = asDBBackupResult(task.result)
                const hasDetail = taskHasDetail(task)
                return (
                  <TableRow key={task.task_id} className='hover:bg-muted/30'>
                    <TableCell className='px-4 py-3 align-middle'>
                      <Badge
                        variant={STATUS_VARIANT[task.status]}
                        className={cn('gap-1.5', STATUS_CLASS_NAME[task.status])}
                      >
                        <span
                          className={cn(
                            'size-1.5 rounded-full',
                            STATUS_DOT_CLASS_NAME[task.status]
                          )}
                          aria-hidden='true'
                        />
                        {t(task.status)}
                      </Badge>
                    </TableCell>
                    <TableCell className='py-3 align-middle text-sm'>
                      {triggerSourceLabel(task, t)}
                    </TableCell>
                    <TableCell className='text-muted-foreground max-w-[240px] truncate py-3 align-middle font-mono text-xs'>
                      {task.locked_by || result?.host || '-'}
                    </TableCell>
                    <TableCell className='py-3 align-middle text-xs tabular-nums'>
                      {result?.duration_ms != null
                        ? `${(result.duration_ms / 1000).toFixed(1)}s`
                        : '-'}
                    </TableCell>
                    <TableCell
                      className='text-muted-foreground py-3 align-middle text-xs whitespace-nowrap'
                      title={formatTimestampToDate(task.updated_at)}
                    >
                      {formatTimestampRelative(
                        task.updated_at,
                        'seconds',
                        toIntlLocale(i18n.language)
                      )}
                    </TableCell>
                    <TableCell className='py-3 pr-4 align-middle'>
                      {hasDetail ? (
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={() => {
                            setDetailTask(task)
                            setDetailOpen(true)
                          }}
                        >
                          {t('View')}
                        </Button>
                      ) : (
                        <span className='text-muted-foreground text-xs'>-</span>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      <DBBackupDetailDialog
        task={detailTask}
        open={detailOpen}
        onOpenChange={(open) => {
          setDetailOpen(open)
          if (!open) setDetailTask(null)
        }}
      />

      <AlertDialog open={showTriggerConfirm} onOpenChange={setShowTriggerConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm database backup')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This will enqueue a host-side PostgreSQL and ClickHouse backup. The host agent claims and runs it within about one minute.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isStarting}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={isStarting}
              onClick={(event) => {
                event.preventDefault()
                void handleTrigger()
              }}
            >
              {isStarting ? t('Starting...') : t('Start backup')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
