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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { DatabaseBackup } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  getCurrentDBBackupTask,
  getDBBackupConfig,
  getDBBackupScript,
  getSystemTask,
  triggerDBBackup,
  updateDBBackupConfig,
  updateDBBackupScript,
} from '../api'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import type { DBBackupConfig, DBBackupTask } from '../types'

const configSchema = z.object({
  backup_root: z.string().min(1),
  pg_container: z.string().min(1),
  ck_container: z.string().min(1),
  pg_user: z.string().min(1),
  pg_db: z.string().min(1),
  ck_user: z.string().min(1),
  ck_databases: z.string().min(1),
  keep_weekly: z.coerce.number().min(1).max(52),
  log_dir: z.string().min(1),
  script_enabled: z.boolean(),
})

type ConfigFormValues = z.infer<typeof configSchema>

function isActiveDBBackupTask(task: DBBackupTask | null) {
  return task?.status === 'pending' || task?.status === 'running'
}

const emptyConfig: ConfigFormValues = {
  backup_root: '/data/backups/new-api',
  pg_container: 'postgres',
  ck_container: 'clickhouse',
  pg_user: 'newapi',
  pg_db: 'newapi',
  ck_user: 'default',
  ck_databases: 'new_api_logs clash_metrics',
  keep_weekly: 4,
  log_dir: '/var/log/new-api-backup',
  script_enabled: true,
}

export function DBBackupSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [loading, setLoading] = useState(true)
  const [savingConfig, setSavingConfig] = useState(false)
  const [savingScript, setSavingScript] = useState(false)
  const [scriptContent, setScriptContent] = useState('')
  const [scriptSha, setScriptSha] = useState('')
  const [showScriptConfirm, setShowScriptConfirm] = useState(false)
  const [showTriggerConfirm, setShowTriggerConfirm] = useState(false)
  const [isStarting, setIsStarting] = useState(false)
  const [task, setTask] = useState<DBBackupTask | null>(null)

  const form = useForm<ConfigFormValues>({
    resolver: zodResolver(configSchema),
    defaultValues: emptyConfig,
  })

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [configRes, scriptRes, taskRes] = await Promise.all([
          getDBBackupConfig(),
          getDBBackupScript(),
          getCurrentDBBackupTask(),
        ])
        if (cancelled) return
        if (configRes.success && configRes.data) {
          const data = configRes.data
          form.reset({
            backup_root: data.backup_root,
            pg_container: data.pg_container,
            ck_container: data.ck_container,
            pg_user: data.pg_user,
            pg_db: data.pg_db,
            ck_user: data.ck_user,
            ck_databases: data.ck_databases,
            keep_weekly: data.keep_weekly,
            log_dir: data.log_dir,
            script_enabled: data.script_enabled,
          })
          if (data.script_sha256) setScriptSha(data.script_sha256)
        }
        if (scriptRes.success && scriptRes.data) {
          setScriptContent(scriptRes.data.content || '')
          setScriptSha(scriptRes.data.sha256 || '')
        }
        if (taskRes.success && taskRes.data) {
          setTask(taskRes.data)
        }
      } catch {
        toast.error(t('Failed to load database backup settings.'))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [form, t])

  const active = isActiveDBBackupTask(task)
  const taskId = task?.task_id

  useEffect(() => {
    if (!taskId || !active) return
    let cancelled = false
    const interval = window.setInterval(async () => {
      try {
        const res = await getSystemTask(taskId)
        if (cancelled || !res.success || !res.data) return
        const nextTask = res.data as DBBackupTask
        setTask(nextTask)
        if (!isActiveDBBackupTask(nextTask)) {
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

  const onSaveConfig = async (values: ConfigFormValues) => {
    setSavingConfig(true)
    try {
      const payload: DBBackupConfig = {
        ...values,
        keep_weekly: Number(values.keep_weekly),
      }
      const res = await updateDBBackupConfig(payload)
      if (!res.success) {
        throw new Error(res.message || t('Failed to save backup settings.'))
      }
      if (res.data) {
        form.reset({
          backup_root: res.data.backup_root,
          pg_container: res.data.pg_container,
          ck_container: res.data.ck_container,
          pg_user: res.data.pg_user,
          pg_db: res.data.pg_db,
          ck_user: res.data.ck_user,
          ck_databases: res.data.ck_databases,
          keep_weekly: res.data.keep_weekly,
          log_dir: res.data.log_dir,
          script_enabled: res.data.script_enabled,
        })
        if (res.data.script_sha256) setScriptSha(res.data.script_sha256)
      }
      toast.success(t('Backup settings saved.'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save backup settings.')
      )
    } finally {
      setSavingConfig(false)
    }
  }

  const onSaveScript = async () => {
    setSavingScript(true)
    try {
      const res = await updateDBBackupScript(scriptContent, true)
      if (!res.success) {
        throw new Error(res.message || t('Failed to save backup script.'))
      }
      setScriptSha(res.data?.sha256 || '')
      setShowScriptConfirm(false)
      toast.success(t('Backup script saved. Host agent will apply it on next poll.'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save backup script.')
      )
    } finally {
      setSavingScript(false)
    }
  }

  const handleTrigger = async () => {
    setIsStarting(true)
    try {
      const res = await triggerDBBackup()
      if (!res.success || !res.data) {
        throw new Error(res.message || t('Failed to start database backup.'))
      }
      setTask(res.data)
      setShowTriggerConfirm(false)
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

  if (loading) {
    return (
      <SettingsSection title={t('Database Backup')}>
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      </SettingsSection>
    )
  }

  return (
    <SettingsSection title={t('Database Backup')}>
      <Alert>
        <AlertDescription>
          {t(
            'Host-side PostgreSQL and ClickHouse backup. Secrets (ClickHouse password, agent token) stay on the host and are never stored here. Editing the script can execute as root on the host.'
          )}
        </AlertDescription>
      </Alert>

      <SettingsForm>
        <Form {...form}>
          <form
            className='flex flex-col gap-4'
            onSubmit={form.handleSubmit(onSaveConfig)}
          >
            <div className='grid gap-4 md:grid-cols-2'>
              {(
                [
                  ['backup_root', 'Backup root path'],
                  ['log_dir', 'Log directory'],
                  ['pg_container', 'PostgreSQL container'],
                  ['ck_container', 'ClickHouse container'],
                  ['pg_user', 'PostgreSQL user'],
                  ['pg_db', 'PostgreSQL database'],
                  ['ck_user', 'ClickHouse user'],
                  ['ck_databases', 'ClickHouse databases'],
                ] as const
              ).map(([name, label]) => (
                <FormField
                  key={name}
                  control={form.control}
                  name={name}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t(label)}</FormLabel>
                      <FormControl>
                        <Input {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ))}
              <FormField
                control={form.control}
                name='keep_weekly'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Weekly backups to keep')}</FormLabel>
                    <FormControl>
                      <Input type='number' min={1} max={52} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('Retention count for weekly backups (1-52).')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='script_enabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                    <div className='space-y-0.5'>
                      <FormLabel>{t('Allow host to apply script')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, the host agent materializes the saved script to a fixed path.'
                        )}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>
            <div className='flex gap-2'>
              <Button type='submit' disabled={savingConfig}>
                {savingConfig ? t('Saving...') : t('Save backup settings')}
              </Button>
            </div>
          </form>
        </Form>
      </SettingsForm>

      <Separator />

      <div className='flex flex-col gap-3'>
        <div className='flex flex-col gap-1'>
          <h4 className='text-sm font-semibold'>{t('Backup script')}</h4>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Empty script keeps the current host file. Saving overwrites /usr/local/bin/backup-new-api-db.sh on next agent poll.'
            )}
          </p>
          {scriptSha ? (
            <p className='text-muted-foreground font-mono text-xs'>
              SHA256: {scriptSha}
            </p>
          ) : null}
        </div>
        <Textarea
          className='min-h-64 font-mono text-xs'
          value={scriptContent}
          onChange={(event) => setScriptContent(event.target.value)}
          spellCheck={false}
        />
        <div>
          <Button
            type='button'
            variant='destructive'
            disabled={savingScript}
            onClick={() => setShowScriptConfirm(true)}
          >
            {t('Save backup script')}
          </Button>
        </div>
      </div>

      <Separator />

      <div className='flex items-center gap-2'>
        <Button
          type='button'
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
        {task?.status ? (
          <span className='text-muted-foreground text-sm'>
            {t('Current status')}: {task.status}
          </span>
        ) : null}
      </div>

      <AlertDialog open={showScriptConfirm} onOpenChange={setShowScriptConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm script update')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This will store a host-executed shell script. Compromised root sessions can achieve host RCE. Continue only if you trust this content.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={savingScript}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={savingScript}
              onClick={(event) => {
                event.preventDefault()
                void onSaveScript()
              }}
            >
              {savingScript ? t('Saving...') : t('Save script')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={showTriggerConfirm}
        onOpenChange={setShowTriggerConfirm}
      >
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
    </SettingsSection>
  )
}
