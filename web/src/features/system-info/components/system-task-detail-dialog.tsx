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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

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
import type {
  DBBackupArtifact,
  DBBackupResult,
  SystemTask,
} from '@/features/system-settings/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatTimestampToDate, formatTimestampRelative } from '@/lib/format'

import { SYSTEM_TASK_TYPE_LABEL as TYPE_LABEL } from '../constants'
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

type SystemTaskDetailDialogProps = {
  task: SystemTask | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SystemTaskDetailDialog({
  task,
  open,
  onOpenChange,
}: SystemTaskDetailDialogProps) {
  const { t, i18n } = useTranslation()
  const backupResult = useMemo(
    () => (task?.type === 'db_backup' ? asDBBackupResult(task.result) : null),
    [task]
  )
  const genericResult = useMemo(() => {
    if (!task?.result) return ''
    if (typeof task.result === 'string') return task.result
    try {
      return JSON.stringify(task.result, null, 2)
    } catch {
      return String(task.result)
    }
  }, [task])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Task detail')}</DialogTitle>
          <DialogDescription>
            {task
              ? `${t(TYPE_LABEL[task.type] ?? task.type)} · ${task.task_id}`
              : t('No task selected.')}
          </DialogDescription>
        </DialogHeader>
        {task ? (
          <div className='space-y-4'>
            <div className='grid gap-3 sm:grid-cols-2'>
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>
                  {t('Status')}
                </div>
                <div className='text-sm font-medium'>{t(task.status)}</div>
              </div>
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>
                  {t('Executor')}
                </div>
                <div className='font-mono text-xs break-all'>
                  {task.locked_by || '-'}
                </div>
              </div>
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>
                  {t('Updated')}
                </div>
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
                <div className='text-muted-foreground text-xs'>
                  {t('Error')}
                </div>
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
                            <TableHead className='text-xs'>
                              {t('Type')}
                            </TableHead>
                            <TableHead className='text-xs'>
                              {t('File')}
                            </TableHead>
                            <TableHead className='text-xs'>
                              {t('Size')}
                            </TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {backupResult.artifacts.map((artifact) => (
                            <TableRow key={`${artifact.type}-${artifact.file}`}>
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
            ) : null}
            {!backupResult && genericResult ? (
              <div className='space-y-1'>
                <div className='text-muted-foreground text-xs'>
                  {t('Result')}
                </div>
                <pre className='bg-muted/40 max-h-72 overflow-auto rounded-md border p-3 font-mono text-[11px] whitespace-pre-wrap'>
                  {genericResult}
                </pre>
              </div>
            ) : null}
            {!backupResult && !genericResult ? (
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
