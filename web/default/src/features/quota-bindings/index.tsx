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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Edit, Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useIsAdmin } from '@/hooks/use-admin'
import { formatTimestampToDate, formatTokens } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  createQuotaBinding,
  deleteQuotaBinding,
  getQuotaBindings,
  refreshQuotaBindingUsage,
  updateQuotaBinding,
} from './api'
import type {
  DeepSeekQuotaBinding,
  GLMQuotaBinding,
  QuotaBinding,
  QuotaBindingFormData,
  QuotaProvider,
} from './types'

type ProviderConfig = {
  provider: QuotaProvider
  titleKey: string
  descriptionKey: string
  curlPlaceholderKey: string
}

const providerConfigs: Record<QuotaProvider, ProviderConfig> = {
  glm: {
    provider: 'glm',
    titleKey: 'GLM Quota',
    descriptionKey: 'Track BigModel account quota usage and refresh it from saved curl requests.',
    curlPlaceholderKey: 'Paste the BigModel usage curl command',
  },
  deepseek: {
    provider: 'deepseek',
    titleKey: 'DeepSeek Quota',
    descriptionKey: 'Track DeepSeek account quota usage and refresh it from saved curl requests.',
    curlPlaceholderKey: 'Paste the DeepSeek quota curl command',
  },
}

const glmPlanSpecs = [
  {
    value: 'standard',
    labelKey: 'Standard',
    fiveHourLimitTokens: 60_000_000,
    weeklyLimitTokens: 300_000_000,
  },
  {
    value: 'advanced',
    labelKey: 'Advanced',
    fiveHourLimitTokens: 160_000_000,
    weeklyLimitTokens: 800_000_000,
  },
] as const

function getGLMPlanLabelKey(value: string): string {
  return (
    glmPlanSpecs.find((plan) => plan.value === value)?.labelKey || value || '-'
  )
}

const emptyForm: QuotaBindingFormData = {
  name: '',
  note: '',
  request_curl: '',
  proxy: '',
  enabled: true,
  plan_type: 'standard',
  five_hour_limit_tokens: 60_000_000,
  weekly_limit_tokens: 300_000_000,
}

function isGLMBinding(binding: QuotaBinding): binding is GLMQuotaBinding {
  return 'last_weekly_used_tokens' in binding
}

function isDeepSeekBinding(
  binding: QuotaBinding
): binding is DeepSeekQuotaBinding {
  return 'last_monthly_used_tokens' in binding
}

function buildForm(binding?: QuotaBinding): QuotaBindingFormData {
  if (!binding) return emptyForm
  return {
    id: binding.id,
    name: binding.name || '',
    note: binding.note || '',
    request_curl: binding.request_curl || '',
    proxy: binding.proxy || '',
    enabled: binding.enabled !== false,
    plan_type: isGLMBinding(binding) ? binding.plan_type || 'standard' : '',
    five_hour_limit_tokens: isGLMBinding(binding)
      ? Number(binding.five_hour_limit_tokens || 0)
      : 0,
    weekly_limit_tokens: isGLMBinding(binding)
      ? Number(binding.weekly_limit_tokens || 0)
      : 0,
  }
}

function normalizePercent(value: number | undefined): number {
  const percent = Number(value || 0)
  if (!Number.isFinite(percent)) return 0
  return Math.min(100, Math.max(0, Math.round(percent)))
}

function progressColor(percent: number): string {
  if (percent >= 90) {
    return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  }
  if (percent >= 70) {
    return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  }
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}

function TokenUsageBar(props: {
  used: number
  limit: number
  percent: number
}) {
  const percent = normalizePercent(props.percent)
  const hasLimit = Number(props.limit || 0) > 0

  return (
    <div className='min-w-[150px] space-y-1'>
      <div className='flex justify-between gap-2 text-xs'>
        <span className='font-mono'>{formatTokens(props.used || 0)}</span>
        <span className='text-muted-foreground font-mono'>
          {hasLimit ? formatTokens(props.limit) : '-'}
        </span>
      </div>
      <Progress
        value={hasLimit ? percent : 0}
        className={cn('h-1.5', progressColor(percent))}
      />
      <div className='text-muted-foreground text-xs'>
        {hasLimit ? `${percent}%` : '-'}
      </div>
    </div>
  )
}

function parseJsonList(value: string | undefined): Array<Record<string, unknown>> {
  if (!value?.trim()) return []
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed)
      ? parsed.filter((item) => item && typeof item === 'object')
      : []
  } catch {
    return []
  }
}

function formatAmount(value: unknown): string {
  const numberValue = Number(value || 0)
  if (!Number.isFinite(numberValue)) return String(value || '0')
  return numberValue.toLocaleString(undefined, {
    maximumFractionDigits: 4,
  })
}

function GLMUsageCells({ binding }: { binding: GLMQuotaBinding }) {
  return (
    <>
      <TableCell>
        <TokenUsageBar
          used={binding.last_five_hour_used_tokens}
          limit={binding.five_hour_limit_tokens}
          percent={binding.last_five_hour_percent}
        />
      </TableCell>
      <TableCell>
        <TokenUsageBar
          used={binding.last_weekly_used_tokens}
          limit={binding.weekly_limit_tokens}
          percent={binding.last_weekly_percent}
        />
      </TableCell>
      <TableCell className='font-mono'>
        {formatTokens(binding.last_model_call_count || 0)}
      </TableCell>
    </>
  )
}

function DeepSeekUsageCells({ binding }: { binding: DeepSeekQuotaBinding }) {
  const wallets = [
    ...parseJsonList(binding.last_normal_wallets),
    ...parseJsonList(binding.last_bonus_wallets),
  ]
  const primaryWallet = wallets[0]
  const walletLabel = primaryWallet
    ? `${String(primaryWallet.currency || '-')} ${formatAmount(primaryWallet.balance)}`
    : '-'

  return (
    <>
      <TableCell>
        <TokenUsageBar
          used={binding.last_monthly_used_tokens}
          limit={binding.last_monthly_limit_tokens}
          percent={binding.last_monthly_percent}
        />
      </TableCell>
      <TableCell className='font-mono'>
        {formatTokens(binding.last_monthly_remaining_tokens || 0)}
      </TableCell>
      <TableCell>{walletLabel}</TableCell>
    </>
  )
}

function ErrorMessageCell({ error }: { error?: string }) {
  if (!error) {
    return <span className='text-muted-foreground'>-</span>
  }

  return (
    <TooltipProvider delay={100}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='text-destructive block max-w-[280px] min-w-0 cursor-default truncate text-xs' />
          }
        >
          {error}
        </TooltipTrigger>
        <TooltipContent
          side='top'
          align='start'
          className='max-w-[min(36rem,calc(100vw-2rem))] whitespace-pre-wrap break-words [overflow-wrap:anywhere]'
        >
          {error}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function getApiError(
  data: { success: boolean; message?: string },
  fallback: string
): string {
  return data.success ? '' : data.message || fallback
}

export function QuotaBindingsPage({ provider }: { provider: QuotaProvider }) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const queryClient = useQueryClient()
  const config = providerConfigs[provider]
  const [keyword, setKeyword] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<QuotaBindingFormData>(emptyForm)
  const [deleteTarget, setDeleteTarget] = useState<QuotaBinding | null>(null)
  const [refreshingId, setRefreshingId] = useState<number | null>(null)
  const [refreshingAll, setRefreshingAll] = useState(false)

  const bindingsQuery = useQuery({
    queryKey: ['quota-bindings', provider, keyword],
    queryFn: () =>
      getQuotaBindings(provider, {
        keyword: keyword.trim() || undefined,
        p: 1,
        page_size: 100,
      }),
  })

  const bindings = useMemo(
    () => bindingsQuery.data?.data?.items || [],
    [bindingsQuery.data?.data?.items]
  )

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['quota-bindings', provider] })

  const saveMutation = useMutation({
    mutationFn: async (data: QuotaBindingFormData) => {
      return data.id
        ? updateQuotaBinding(provider, data.id, data)
        : createQuotaBinding(provider, data)
    },
    onSuccess: (data) => {
      const message = getApiError(data, t('Failed to save quota binding'))
      if (message) {
        toast.error(message)
        return
      }
      toast.success(t('Saved successfully'))
      setDialogOpen(false)
      invalidate()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save quota binding'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (binding: QuotaBinding) =>
      deleteQuotaBinding(provider, binding.id),
    onSuccess: (data) => {
      const message = getApiError(data, t('Failed to delete quota binding'))
      if (message) {
        toast.error(message)
        return
      }
      toast.success(t('Deleted successfully'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to delete quota binding'))
    },
  })

  const openCreateDialog = () => {
    setForm(emptyForm)
    setDialogOpen(true)
  }

  const openEditDialog = (binding: QuotaBinding) => {
    setForm(buildForm(binding))
    setDialogOpen(true)
  }

  const updateForm = (patch: Partial<QuotaBindingFormData>) => {
    setForm((current) => ({ ...current, ...patch }))
  }

  const refreshBinding = async (binding: QuotaBinding) => {
    setRefreshingId(binding.id)
    try {
      const result = await refreshQuotaBindingUsage(provider, binding.id)
      if (!result.success) {
        toast.error(result.message || t('Refresh failed'))
        return
      }
      if (result.data?.last_error) {
        toast.error(result.data.last_error)
      } else {
        toast.success(t('Refresh succeeded'))
      }
      invalidate()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Refresh failed'))
    } finally {
      setRefreshingId(null)
    }
  }

  const refreshAll = async () => {
    const enabledBindings = bindings.filter((binding) => binding.enabled)
    if (enabledBindings.length === 0) {
      toast.error(t('No enabled quota bindings to refresh'))
      return
    }
    setRefreshingAll(true)
    let successCount = 0
    let failCount = 0
    try {
      const results = await Promise.allSettled(
        enabledBindings.map((binding) =>
          refreshQuotaBindingUsage(provider, binding.id)
        )
      )
      for (const result of results) {
        if (
          result.status === 'fulfilled' &&
          result.value.success &&
          !result.value.data?.last_error
        ) {
          successCount++
        } else {
          failCount++
        }
      }
      if (failCount === 0) {
        toast.success(t('Refresh succeeded'))
      } else {
        toast.error(
          t('Refresh completed: {{success}} succeeded, {{fail}} failed', {
            success: successCount,
            fail: failCount,
          })
        )
      }
      invalidate()
    } finally {
      setRefreshingAll(false)
    }
  }

  const submitForm = () => {
    if (!form.name.trim()) {
      toast.error(t('Name is required'))
      return
    }
    if (!form.id && !form.request_curl.trim()) {
      toast.error(t('Curl command is required'))
      return
    }
    if (provider === 'glm') {
      if (!form.plan_type.trim()) {
        toast.error(t('Plan is required'))
        return
      }
      if (form.five_hour_limit_tokens <= 0 || form.weekly_limit_tokens <= 0) {
        toast.error(t('Quota limits are required'))
        return
      }
    }
    saveMutation.mutate({
      ...form,
      name: form.name.trim(),
      note: form.note.trim(),
      request_curl: form.request_curl.trim(),
      proxy: form.proxy.trim(),
    })
  }

  const applyGLMPlan = (value: string) => {
    const spec = glmPlanSpecs.find((item) => item.value === value)
    updateForm({
      plan_type: value,
      five_hour_limit_tokens:
        spec?.fiveHourLimitTokens || form.five_hour_limit_tokens,
      weekly_limit_tokens: spec?.weeklyLimitTokens || form.weekly_limit_tokens,
    })
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t(config.titleKey)}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          onClick={refreshAll}
          disabled={refreshingAll || bindingsQuery.isLoading}
        >
          <RefreshCw className={refreshingAll ? 'animate-spin' : undefined} />
          {t('Refresh All')}
        </Button>
        {isAdmin && (
          <Button onClick={openCreateDialog}>
            <Plus />
            {t('Create')}
          </Button>
        )}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Card>
          <CardHeader>
            <CardTitle>{t(config.titleKey)}</CardTitle>
            <CardDescription>{t(config.descriptionKey)}</CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='max-w-sm'>
              <Input
                value={keyword}
                placeholder={t('Search by name or note')}
                onChange={(event) => setKeyword(event.target.value)}
              />
            </div>
            {bindingsQuery.data && !bindingsQuery.data.success ? (
              <Alert variant='destructive'>
                <AlertDescription>
                  {bindingsQuery.data.message ||
                    t('Failed to load quota bindings')}
                </AlertDescription>
              </Alert>
            ) : null}
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Name')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  {provider === 'glm' ? (
                    <>
                      <TableHead>{t('Five-hour Tokens')}</TableHead>
                      <TableHead>{t('Weekly Tokens')}</TableHead>
                      <TableHead>{t('Model Calls')}</TableHead>
                    </>
                  ) : (
                    <>
                      <TableHead>{t('Monthly Tokens')}</TableHead>
                      <TableHead>{t('Remaining Tokens')}</TableHead>
                      <TableHead>{t('Wallet')}</TableHead>
                    </>
                  )}
                  <TableHead>{t('Last Refreshed')}</TableHead>
                  <TableHead className='w-[300px] max-w-[300px]'>
                    {t('Error')}
                  </TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {bindings.map((binding) => (
                  <TableRow key={binding.id}>
                    <TableCell>
                      <div className='flex flex-col gap-1'>
                        <span className='font-medium'>{binding.name}</span>
                        {binding.note ? (
                          <span className='text-muted-foreground max-w-[260px] truncate text-xs'>
                            {binding.note}
                          </span>
                        ) : null}
                        {isGLMBinding(binding) && binding.plan_type ? (
                          <Badge variant='outline' className='w-fit'>
                            {t(getGLMPlanLabelKey(binding.plan_type))}
                          </Badge>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={binding.enabled ? 'default' : 'outline'}>
                        {binding.enabled ? t('Enabled') : t('Disabled')}
                      </Badge>
                    </TableCell>
                    {isGLMBinding(binding) ? (
                      <GLMUsageCells binding={binding} />
                    ) : isDeepSeekBinding(binding) ? (
                      <DeepSeekUsageCells binding={binding} />
                    ) : null}
                    <TableCell>
                      {binding.last_refreshed_at
                        ? formatTimestampToDate(binding.last_refreshed_at)
                        : '-'}
                    </TableCell>
                    <TableCell className='w-[300px] max-w-[300px] min-w-0'>
                      <ErrorMessageCell error={binding.last_error} />
                    </TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => refreshBinding(binding)}
                          disabled={
                            !binding.enabled || refreshingId === binding.id
                          }
                          aria-label={t('Refresh')}
                        >
                          <RefreshCw
                            className={
                              refreshingId === binding.id
                                ? 'animate-spin'
                                : undefined
                            }
                          />
                        </Button>
                        {isAdmin && (
                          <>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => openEditDialog(binding)}
                              aria-label={t('Edit')}
                            >
                              <Edit />
                            </Button>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => setDeleteTarget(binding)}
                              aria-label={t('Delete')}
                            >
                              <Trash2 />
                            </Button>
                          </>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
                {bindings.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={8} className='h-24 text-center'>
                      {bindingsQuery.isLoading
                        ? t('Loading...')
                        : t('No quota bindings found')}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </SectionPageLayout.Content>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {form.id ? t('Edit Quota Binding') : t('Create Quota Binding')}
            </DialogTitle>
            <DialogDescription>
              {t('Save the curl command and optional proxy used to refresh quota usage.')}
            </DialogDescription>
          </DialogHeader>
          <div className='grid max-h-[70vh] gap-4 overflow-y-auto pr-1'>
            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='space-y-2'>
                <Label>{t('Name')}</Label>
                <Input
                  value={form.name}
                  onChange={(event) => updateForm({ name: event.target.value })}
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Proxy')}</Label>
                <Input
                  value={form.proxy}
                  placeholder='http://127.0.0.1:7990'
                  onChange={(event) =>
                    updateForm({ proxy: event.target.value })
                  }
                />
              </div>
            </div>

            {provider === 'glm' && (
              <div className='grid gap-4 sm:grid-cols-3'>
                <div className='space-y-2'>
                  <Label>{t('Plan')}</Label>
                  <select
                    value={form.plan_type}
                    onChange={(event) => applyGLMPlan(event.target.value)}
                    className='border-input bg-background h-9 w-full rounded-lg border px-2 text-sm'
                  >
                    {glmPlanSpecs.map((plan) => (
                      <option key={plan.value} value={plan.value}>
                        {t(plan.labelKey)}
                      </option>
                    ))}
                  </select>
                </div>
                <div className='space-y-2'>
                  <Label>{t('Five-hour Tokens')}</Label>
                  <Input
                    type='number'
                    min={0}
                    value={form.five_hour_limit_tokens}
                    onChange={(event) =>
                      updateForm({
                        five_hour_limit_tokens: Number(
                          event.target.value || 0
                        ),
                      })
                    }
                  />
                </div>
                <div className='space-y-2'>
                  <Label>{t('Weekly Tokens')}</Label>
                  <Input
                    type='number'
                    min={0}
                    value={form.weekly_limit_tokens}
                    onChange={(event) =>
                      updateForm({
                        weekly_limit_tokens: Number(event.target.value || 0),
                      })
                    }
                  />
                </div>
              </div>
            )}

            <div className='space-y-2'>
              <Label>{t('Curl Command')}</Label>
              <Textarea
                value={form.request_curl}
                placeholder={t(config.curlPlaceholderKey)}
                onChange={(event) =>
                  updateForm({ request_curl: event.target.value })
                }
              />
            </div>

            <div className='space-y-2'>
              <Label>{t('Note')}</Label>
              <Textarea
                value={form.note}
                onChange={(event) => updateForm({ note: event.target.value })}
              />
            </div>

            <label className='flex items-center gap-3'>
              <Switch
                checked={form.enabled}
                onCheckedChange={(checked) =>
                  updateForm({ enabled: checked === true })
                }
              />
              <span className='text-sm font-medium'>{t('Enabled')}</span>
            </label>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              type='button'
              onClick={() => setDialogOpen(false)}
              disabled={saveMutation.isPending}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              onClick={submitForm}
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending ? (
                <Loader2 className='animate-spin' />
              ) : null}
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Quota Binding')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This action cannot be undone.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={deleteMutation.isPending || !deleteTarget}
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget)}
            >
              {deleteMutation.isPending ? (
                <Loader2 className='animate-spin' />
              ) : null}
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SectionPageLayout>
  )
}
