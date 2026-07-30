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
import { useEffect, useMemo, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
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
import { cn } from '@/lib/utils'

import {
  getDBBackupConfig,
  getDBBackupScript,
  updateDBBackupConfig,
  updateDBBackupScript,
} from '../api'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import type { DBBackupConfig } from '../types'
import { DBBackupHistoryPanel } from './db-backup-history-panel'

const BACKUP_CRON_PRESETS = [
  {
    id: 'daily-3am',
    expression: '0 3 * * *',
    label: 'Every day at 03:00',
  },
  {
    id: 'sunday-3am',
    expression: '0 3 * * 0',
    label: 'Every Sunday at 03:00',
  },
  {
    id: 'monday-3am',
    expression: '0 3 * * 1',
    label: 'Every Monday at 03:00',
  },
  {
    id: 'month-1st-3am',
    expression: '0 3 1 * *',
    label: '1st of each month at 03:00',
  },
  {
    id: 'every-12h',
    expression: '0 */12 * * *',
    label: 'Every 12 hours',
  },
] as const

const CRON_FIELD_GUIDE = [
  { field: 'Cron minute', range: '0-59', example: '0' },
  { field: 'Cron hour', range: '0-23', example: '3' },
  { field: 'Cron day of month', range: '1-31', example: '*' },
  { field: 'Cron month', range: '1-12', example: '*' },
  { field: 'Cron day of week', range: '0-6 (Sun=0)', example: '0' },
] as const

function normalizeCronExpression(value: string) {
  return value.trim().replace(/\s+/g, ' ')
}

function matchBackupCronPreset(expression: string) {
  const normalized = normalizeCronExpression(expression)
  return (
    BACKUP_CRON_PRESETS.find((preset) => preset.expression === normalized) ??
    null
  )
}

const configSchema = z
  .object({
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
    schedule_enabled: z.boolean(),
    cron_expression: z.string().max(128),
  })
  .superRefine((values, ctx) => {
    if (!values.schedule_enabled) return
    const expr = values.cron_expression.trim()
    if (!expr) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['cron_expression'],
        message: 'Cron expression is required when schedule is enabled',
      })
      return
    }
    // Basic 5-field shape check; server re-validates with robfig/cron.
    if (expr.split(/\s+/).length !== 5) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['cron_expression'],
        message: 'Use a standard 5-field cron expression',
      })
    }
  })

type ConfigFormValues = z.infer<typeof configSchema>

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
  schedule_enabled: false,
  cron_expression: '0 3 * * 0',
}

export function DBBackupSection() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [savingConfig, setSavingConfig] = useState(false)
  const [savingScript, setSavingScript] = useState(false)
  const [scriptContent, setScriptContent] = useState('')
  const [scriptSha, setScriptSha] = useState('')
  const [scriptIsDefault, setScriptIsDefault] = useState(false)
  const [showScriptConfirm, setShowScriptConfirm] = useState(false)

  const form = useForm<ConfigFormValues>({
    resolver: zodResolver(configSchema),
    defaultValues: emptyConfig,
  })
  const scheduleEnabled = useWatch({
    control: form.control,
    name: 'schedule_enabled',
  })
  const cronExpression = useWatch({
    control: form.control,
    name: 'cron_expression',
  })
  const matchedCronPreset = useMemo(
    () => matchBackupCronPreset(cronExpression || ''),
    [cronExpression]
  )

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [configRes, scriptRes] = await Promise.all([
          getDBBackupConfig(),
          getDBBackupScript(),
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
            schedule_enabled: Boolean(data.schedule_enabled),
            cron_expression: data.cron_expression || '0 3 * * 0',
          })
          if (data.script_sha256) setScriptSha(data.script_sha256)
        }
        if (scriptRes.success && scriptRes.data) {
          setScriptContent(scriptRes.data.content || '')
          setScriptSha(scriptRes.data.sha256 || '')
          setScriptIsDefault(Boolean(scriptRes.data.is_default))
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
          schedule_enabled: Boolean(res.data.schedule_enabled),
          cron_expression: res.data.cron_expression || '0 3 * * 0',
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
      setScriptIsDefault(false)
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
              <FormField
                control={form.control}
                name='schedule_enabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3 md:col-span-2'>
                    <div className='space-y-0.5'>
                      <FormLabel>{t('Enable scheduled backup')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, new-api enqueues a host backup on the cron schedule. The host agent still executes the dump.'
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
              <FormField
                control={form.control}
                name='cron_expression'
                render={({ field }) => {
                  const disabled = !scheduleEnabled
                  return (
                    <FormItem className='md:col-span-2'>
                      <FormLabel>{t('Backup schedule')}</FormLabel>
                      <div
                        className={cn(
                          'space-y-3 rounded-lg border p-3',
                          disabled && 'opacity-60'
                        )}
                      >
                        <div className='space-y-2'>
                          <div className='text-muted-foreground text-xs font-medium'>
                            {t('Common schedules')}
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {BACKUP_CRON_PRESETS.map((preset) => {
                              const active =
                                matchedCronPreset?.id === preset.id
                              return (
                                <Button
                                  key={preset.id}
                                  type='button'
                                  size='sm'
                                  variant={active ? 'default' : 'outline'}
                                  disabled={disabled}
                                  className='h-8'
                                  onClick={() => {
                                    field.onChange(preset.expression)
                                    form.clearErrors('cron_expression')
                                  }}
                                >
                                  {t(preset.label)}
                                </Button>
                              )
                            })}
                          </div>
                        </div>

                        <div className='space-y-2'>
                          <div className='text-muted-foreground text-xs font-medium'>
                            {t('Cron expression')}
                          </div>
                          <FormControl>
                            <Input
                              {...field}
                              value={field.value}
                              onChange={(event) => {
                                field.onChange(event.target.value)
                              }}
                              placeholder='0 3 * * 0'
                              className='font-mono'
                              disabled={disabled}
                              spellCheck={false}
                              autoComplete='off'
                            />
                          </FormControl>
                          <div className='text-muted-foreground font-mono text-xs'>
                            {matchedCronPreset
                              ? `${t(matchedCronPreset.label)} · ${matchedCronPreset.expression}`
                              : field.value?.trim()
                                ? `${t('Custom expression')} · ${normalizeCronExpression(field.value)}`
                                : t('Pick a preset or enter a 5-field cron')}
                          </div>
                        </div>

                        <div className='overflow-x-auto rounded-md border'>
                          <table className='w-full min-w-[420px] text-left text-xs'>
                            <thead className='bg-muted/40 text-muted-foreground'>
                              <tr>
                                <th className='px-2 py-1.5 font-medium'>
                                  {t('Cron field')}
                                </th>
                                <th className='px-2 py-1.5 font-medium'>
                                  {t('Allowed values')}
                                </th>
                                <th className='px-2 py-1.5 font-medium'>
                                  {t('Sample value')}
                                </th>
                              </tr>
                            </thead>
                            <tbody>
                              {CRON_FIELD_GUIDE.map((item) => (
                                <tr
                                  key={item.field}
                                  className='border-t border-border/60'
                                >
                                  <td className='px-2 py-1.5 font-mono'>
                                    {t(item.field)}
                                  </td>
                                  <td className='px-2 py-1.5 font-mono'>
                                    {item.range}
                                  </td>
                                  <td className='px-2 py-1.5 font-mono'>
                                    {item.example}
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>

                        <FormDescription>
                          {t(
                            'Standard 5-field cron (minute hour day month weekday), local server time. Prefer presets; disable any host crontab that also dumps the same databases.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </div>
                    </FormItem>
                  )
                }}
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
          {scriptIsDefault ? (
            <Alert>
              <AlertDescription>
                {t(
                  'Showing the default template. Saving will materialize it on the host agent.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}
          {scriptSha ? (
            <p className='text-muted-foreground font-mono text-xs'>
              SHA256: {scriptSha}
            </p>
          ) : null}
        </div>
        <Textarea
          className='min-h-64 font-mono text-xs'
          value={scriptContent}
          onChange={(event) => {
            setScriptContent(event.target.value)
            if (scriptIsDefault) setScriptIsDefault(false)
          }}
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

      <DBBackupHistoryPanel />

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
    </SettingsSection>
  )
}
