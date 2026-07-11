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
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Check,
  Edit,
  Loader2,
  Plus,
  RefreshCw,
  Search,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  formatTimestampToDate,
  formatTokenDetails,
  formatTokens,
} from '@/lib/format'
import { cn } from '@/lib/utils'
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
  CardAction,
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
import { SectionPageLayout } from '@/components/layout'
import { useSystemOptions } from '@/features/system-settings/hooks/use-system-options'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import { searchUsers } from '@/features/users/api'
import type { User } from '@/features/users/types'
import { useIsAdmin } from '@/hooks/use-admin'
import {
  createCliproxyAuthFileBinding,
  deleteCliproxyAuthFileBinding,
  getCliproxyAuthFileBindings,
  getCliproxyRemoteAuthFiles,
  refreshCliproxyAuthFileBindingUsage,
  toBindingFormData,
  updateCliproxyAuthFileBinding,
} from './api'
import {
  buildCliproxyUsageSummary,
  buildCliproxyXAIUsageSummary,
  type CliproxyXAIUsageWindow,
  type CliproxyUsageWindowKey,
} from './lib/usage-summary'
import { refreshCliproxyAuthFileBindingsUsageAll } from './lib/bulk-refresh'
import {
  getCliproxyAuthFileEmail,
  getCliproxyAuthFileType,
  getCliproxyAuthFileTypeLabel,
} from './lib/auth-file-type'
import type {
  CliproxyAuthFile,
  CliproxyAuthFileBinding,
  CliproxyAuthFileBindingFormData,
} from './types'

type BindingDialogState =
  | { open: false; mode: 'create'; authFile?: undefined; binding?: undefined }
  | {
      open: true
      mode: 'create'
      authFile: CliproxyAuthFile
      binding?: undefined
    }
  | {
      open: true
      mode: 'edit'
      authFile?: undefined
      binding: CliproxyAuthFileBinding
    }

type BindingFormState = CliproxyAuthFileBindingFormData & {
  username: string
}

const emptyBindingForm: BindingFormState = {
  user_id: 0,
  username: '',
  auth_index: '',
  auth_name: '',
  auth_file: '',
  description: '',
  account_id: '',
  last_plan_type: '',
  enabled: true,
}

type PlanLabelConfig = {
  label: string
  multiplier?: string
  className: string
}

const normalizePlanKey = (value: unknown): string => {
  if (typeof value !== 'string') return ''
  return value
    .trim()
    .toLowerCase()
    .replaceAll(/[-_\s]/g, '')
}

const getPlanLabelConfig = (value: unknown): PlanLabelConfig | null => {
  const key = normalizePlanKey(value)
  if (!key) return null

  if (key === 'pro' || key === 'pro20x' || key === 'planmax' || key === 'claudemax') {
    return {
      label: 'Pro',
      multiplier: '20x',
      className:
        'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-950/35 dark:text-amber-200',
    }
  }

  if (key === 'prolite' || key === 'pro5x') {
    return {
      label: 'Pro',
      multiplier: '5x',
      className:
        'border-sky-300 bg-sky-50 text-sky-800 dark:border-sky-700 dark:bg-sky-950/35 dark:text-sky-200',
    }
  }

  if (key === 'team' || key === 'planteam' || key === 'claudeteam') {
    return {
      label: 'Team',
      className:
        'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/35 dark:text-emerald-200',
    }
  }

  if (key === 'plus' || key === 'planpro' || key === 'claudepro') {
    return {
      label: 'Plus',
      className:
        'border-indigo-300 bg-indigo-50 text-indigo-800 dark:border-indigo-700 dark:bg-indigo-950/35 dark:text-indigo-200',
    }
  }

  if (key === 'free' || key === 'planfree' || key === 'claudefree') {
    return {
      label: 'Free',
      className: 'border-border bg-muted text-muted-foreground',
    }
  }

  if (key === 'supergrok' || key === 'supergrokheavy') {
    return {
      label: 'SuperGrok',
      className:
        'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/35 dark:text-emerald-200',
    }
  }

  return {
    label: String(value),
    className: 'border-border bg-background text-muted-foreground',
  }
}

function PlanLabel(props: { value?: string | null }) {
  const config = getPlanLabelConfig(props.value)
  if (!config) {
    return <span className='text-muted-foreground'>-</span>
  }

  return (
    <Badge
      variant='outline'
      className={cn(
        'inline-flex h-6 items-center gap-1 rounded-md px-2 font-mono text-[11px] font-semibold tracking-normal',
        config.className
      )}
    >
      <span>{config.label}</span>
      {config.multiplier ? (
        <span className='rounded-[3px] bg-current/10 px-1 text-[10px] leading-4'>
          {config.multiplier}
        </span>
      ) : null}
    </Badge>
  )
}

function AuthFileTypeBadge(props: { binding: CliproxyAuthFileBinding }) {
  return <AuthFileTypeLabel source={props.binding} />
}

function AuthFileTypeLabel(props: {
  source: {
    auth_name?: string
    auth_file?: string
    last_plan_type?: string
  }
}) {
  const type = getCliproxyAuthFileType(props.source)
  const label = getCliproxyAuthFileTypeLabel(type)
  let className =
    'border-sky-300 bg-sky-50 text-sky-800 dark:border-sky-700 dark:bg-sky-950/35 dark:text-sky-200'
  if (type === 'claude') {
    className =
      'border-violet-300 bg-violet-50 text-violet-800 dark:border-violet-700 dark:bg-violet-950/35 dark:text-violet-200'
  } else if (type === 'xai') {
    className =
      'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/35 dark:text-emerald-200'
  }

  return (
    <Badge
      variant='outline'
      className={cn(
        'inline-flex h-6 rounded-md px-2 font-mono text-[11px] font-semibold tracking-normal',
        className
      )}
    >
      {label}
    </Badge>
  )
}

function RemoteAuthFilePlanCell(props: { authFile: CliproxyAuthFile }) {
  const source = {
    auth_name: props.authFile.name,
    auth_file: props.authFile.authFile,
    last_plan_type: props.authFile.planType,
  }

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <AuthFileTypeLabel source={source} />
      {normalizePlanKey(props.authFile.planType) === 'claude' ? null : (
        <PlanLabel value={props.authFile.planType} />
      )}
    </div>
  )
}

function AuthFileEmailCell(props: { binding: CliproxyAuthFileBinding }) {
  const email = getCliproxyAuthFileEmail(props.binding)
  return (
    <div className='max-w-[260px] truncate font-mono text-sm'>
      {email || '-'}
    </div>
  )
}

function normalizeUsagePercent(value: unknown): number {
  const percent = Number(value || 0)
  if (!Number.isFinite(percent)) return 0
  return Math.min(100, Math.max(0, Math.round(percent)))
}

function usageProgressColor(percent: number): string {
  if (percent >= 90) {
    return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  }
  if (percent >= 70) {
    return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  }
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}

function UsageLimitBar({
  label,
  percent,
}: {
  label: string
  percent: number
}) {
  const normalizedPercent = normalizeUsagePercent(percent)

  return (
    <div className='min-w-[140px] space-y-1'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='text-muted-foreground'>{label}</span>
        <span className='font-mono font-medium'>{normalizedPercent}%</span>
      </div>
      <Progress
        value={normalizedPercent}
        className={cn('h-1.5', usageProgressColor(normalizedPercent))}
      />
    </div>
  )
}

function usageWindowLabel(
  key: CliproxyUsageWindowKey,
  labels: {
    fiveHour: string
    weekly: string
    codexFiveHour: string
    codexWeekly: string
  }
): string {
  if (key === 'fiveHour') return labels.fiveHour
  if (key === 'weekly') return labels.weekly
  if (key === 'codexFiveHour') return labels.codexFiveHour
  return labels.codexWeekly
}

function xaiUsageWindowLabel(
  key: CliproxyXAIUsageWindow['key'],
  labels: {
    weekly: string
    api: string
    monthly: string
  }
): string {
  if (key === 'weekly') return labels.weekly
  if (key === 'api') return labels.api
  return labels.monthly
}

function BindingUsageCell({
  binding,
  labels,
}: {
  binding: CliproxyAuthFileBinding
  labels: {
    fiveHour: string
    weekly: string
    codexFiveHour: string
    codexWeekly: string
    reset: string
    quota: string
  }
}) {
  const { t } = useTranslation()
  const type = getCliproxyAuthFileType(binding)
  if (type === 'xai') {
    const xaiSummary = buildCliproxyXAIUsageSummary(binding)
    const xaiUsageLabels = {
      weekly: t('Weekly Limit'),
      api: t('{{product}} Usage', { product: 'Api' }),
      monthly: t('Monthly Billing Limit'),
    }

    return (
      <div className='min-w-[300px] space-y-2'>
        <div className='grid gap-2 sm:grid-cols-2'>
          {xaiSummary.primaryWindows.map((window) => (
            <UsageLimitBar
              key={window.key}
              label={xaiUsageWindowLabel(window.key, xaiUsageLabels)}
              percent={window.percent}
            />
          ))}
        </div>
        <div className='flex flex-wrap items-center gap-x-3 gap-y-1'>
          <PlanLabel value={binding.last_plan_type || 'SuperGrok'} />
          {binding.last_error ? (
            <Badge variant='destructive' className='h-5 px-1.5 text-[11px]'>
              {t('Error')}
            </Badge>
          ) : null}
          <span className='text-muted-foreground text-xs'>
            {t('On-demand Cap')} {xaiSummary.onDemandCapLabel}
          </span>
          <span className='text-muted-foreground text-xs'>
            {xaiSummary.billingPeriodEndAt > 0
              ? formatTimestampToDate(xaiSummary.billingPeriodEndAt)
              : '-'}
          </span>
        </div>
      </div>
    )
  }

  const summary = buildCliproxyUsageSummary(binding)

  if (!summary.hasUsageWindow) {
    return (
      <div>
        <TooltipProvider delay={150}>
          <Tooltip>
            <TooltipTrigger render={<div className='cursor-default' />}>
              {formatTokens(binding.last_usage_tokens)}
            </TooltipTrigger>
            <TooltipContent>
              {formatTokenDetails(binding.last_usage_tokens)}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <div className='text-muted-foreground text-xs'>
          {labels.quota} {binding.last_usage_quota || '-'}
        </div>
        {binding.last_plan_type ? (
          <div className='mt-1'>
            <PlanLabel value={binding.last_plan_type} />
          </div>
        ) : null}
      </div>
    )
  }

  return (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger
          render={<div className='max-w-[460px] cursor-help space-y-2' />}
        >
          <div className='grid gap-2 sm:grid-cols-2'>
            {summary.primaryWindows.map((window) => (
              <UsageLimitBar
                key={window.key}
                label={usageWindowLabel(window.key, labels)}
                percent={window.percent}
              />
            ))}
          </div>
          {binding.last_plan_type || binding.last_error ? (
            <div className='flex items-center gap-2'>
              {binding.last_plan_type ? (
                <PlanLabel value={binding.last_plan_type} />
              ) : null}
              {binding.last_error ? (
                <Badge
                  variant='destructive'
                  className='h-5 px-1.5 text-[11px]'
                >
                  {t('Error')}
                </Badge>
              ) : null}
            </div>
          ) : null}
        </TooltipTrigger>
        <TooltipContent
          side='top'
          align='start'
          className='max-w-[min(34rem,calc(100vw-2rem))] whitespace-normal p-3'
        >
          <div className='grid gap-2'>
            {summary.detailWindows.map((window) => (
              <div
                key={window.key}
                className='grid grid-cols-[minmax(8rem,1fr)_auto] gap-x-4 gap-y-0.5'
              >
                <span>{usageWindowLabel(window.key, labels)}</span>
                <span className='font-mono font-semibold'>
                  {normalizeUsagePercent(window.percent)}%
                </span>
                <span className='text-background/70 col-span-2'>
                  {labels.reset}:{' '}
                  {window.resetAt > 0
                    ? formatTimestampToDate(window.resetAt)
                    : '-'}
                </span>
              </div>
            ))}
            {binding.last_error ? (
              <div className='border-background/15 border-t pt-2'>
                <div className='text-background/80 whitespace-pre-wrap break-words'>
                  {binding.last_error}
                </div>
              </div>
            ) : null}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

const cliproxyOptionDefaults = {
  CliproxyAPIBaseURL: '',
  CliproxyAPIPassword: '',
}

function buildFormFromDialog(state: BindingDialogState): BindingFormState {
  if (!state.open) return emptyBindingForm
  if (state.mode === 'edit') {
    const { binding } = state
    return {
      user_id: binding.user_id,
      username: binding.username,
      auth_index: binding.auth_index,
      auth_name: binding.auth_name,
      auth_file: binding.auth_file,
      description: binding.description,
      account_id: binding.account_id,
      last_plan_type: binding.last_plan_type,
      enabled: binding.enabled,
    }
  }

  return {
    ...toBindingFormData(state.authFile, 0),
    username: '',
  }
}

function getApiErrorMessage(
  data: { success: boolean; message?: string },
  fallback: string
) {
  return data.success ? '' : data.message || fallback
}

function ConfigCard() {
  const { t } = useTranslation()
  const optionsQuery = useSystemOptions()
  const updateOption = useUpdateOption()

  const options = useMemo(() => {
    const result = { ...cliproxyOptionDefaults }
    optionsQuery.data?.data?.forEach((option) => {
      if (option.key in result) {
        result[option.key as keyof typeof result] = option.value
      }
    })
    return result
  }, [optionsQuery.data?.data])

  const [baseURL, setBaseURL] = useState(options.CliproxyAPIBaseURL)
  const [password, setPassword] = useState('')

  useEffect(() => {
    setBaseURL(options.CliproxyAPIBaseURL)
  }, [options.CliproxyAPIBaseURL])

  const saveConfig = async () => {
    const trimmedBaseURL = baseURL.trim()
    if (!trimmedBaseURL) {
      toast.error(t('Cliproxy API base URL is required'))
      return
    }

    await updateOption.mutateAsync({
      key: 'CliproxyAPIBaseURL',
      value: trimmedBaseURL,
    })

    if (password.trim()) {
      await updateOption.mutateAsync({
        key: 'CliproxyAPIPassword',
        value: password.trim(),
      })
      setPassword('')
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Cliproxy API Configuration')}</CardTitle>
        <CardDescription>
          {t(
            'Configure the Cliproxy API address and login password for management requests.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className='grid gap-4 lg:grid-cols-[1fr_1fr_auto] lg:items-end'>
          <div className='space-y-2'>
            <Label htmlFor='cliproxy-api-base-url'>
              {t('Cliproxy API Base URL')}
            </Label>
            <Input
              id='cliproxy-api-base-url'
              value={baseURL}
              placeholder='http://127.0.0.1:8317'
              onChange={(event) => setBaseURL(event.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='cliproxy-api-password'>
              {t('Cliproxy API Login Password')}
            </Label>
            <Input
              id='cliproxy-api-password'
              type='password'
              value={password}
              placeholder={t('Leave blank to keep unchanged')}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>
          <Button
            type='button'
            onClick={saveConfig}
            disabled={updateOption.isPending || optionsQuery.isLoading}
          >
            {updateOption.isPending ? (
              <Loader2 className='animate-spin' />
            ) : null}
            {t('Save')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function BindingDialog({
  state,
  onOpenChange,
}: {
  state: BindingDialogState
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState(() => buildFormFromDialog(state))
  const [userKeyword, setUserKeyword] = useState('')

  useEffect(() => {
    setForm(buildFormFromDialog(state))
    setUserKeyword('')
  }, [state])

  const usersQuery = useQuery({
    queryKey: ['cliproxy-auth-files', 'users', userKeyword],
    queryFn: () => searchUsers({ keyword: userKeyword, page_size: 8 }),
    enabled: state.open,
  })

  const saveMutation = useMutation({
    mutationFn: async (data: CliproxyAuthFileBindingFormData) => {
      if (state.open && state.mode === 'edit') {
        return updateCliproxyAuthFileBinding(state.binding.id, data)
      }
      return createCliproxyAuthFileBinding(data)
    },
    onSuccess: (data) => {
      const errorMessage = getApiErrorMessage(data, t('Failed to save binding'))
      if (errorMessage) {
        toast.error(errorMessage)
        return
      }
      toast.success(t('Binding saved successfully'))
      queryClient.invalidateQueries({
        queryKey: ['cliproxy-auth-file-bindings'],
      })
      onOpenChange(false)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save binding'))
    },
  })

  const users = usersQuery.data?.data?.items ?? []

  const updateForm = (patch: Partial<BindingFormState>) => {
    setForm((current) => ({ ...current, ...patch }))
  }

  const selectUser = (user: User) => {
    updateForm({ user_id: user.id, username: user.username })
  }

  const submit = () => {
    if (!form.user_id) {
      toast.error(t('Please select a user'))
      return
    }
    if (!form.auth_index.trim()) {
      toast.error(t('Auth index is required'))
      return
    }

    saveMutation.mutate({
      user_id: form.user_id,
      auth_index: form.auth_index.trim(),
      auth_name: form.auth_name.trim(),
      auth_file: form.auth_file,
      description: form.description,
      account_id: form.account_id.trim(),
      last_plan_type: form.last_plan_type.trim(),
      enabled: form.enabled,
    })
  }

  return (
    <Dialog open={state.open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {state.open && state.mode === 'edit'
              ? t('Edit Binding')
              : t('Create Binding')}
          </DialogTitle>
          <DialogDescription>
            {t('Bind a Cliproxy auth file to a new-api user.')}
          </DialogDescription>
        </DialogHeader>

        <div className='grid max-h-[70vh] gap-4 overflow-y-auto pr-1'>
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('Auth Index')}</Label>
              <Input
                value={form.auth_index}
                onChange={(event) =>
                  updateForm({ auth_index: event.target.value })
                }
              />
            </div>
            <div className='space-y-2'>
              <Label>{t('Auth Name')}</Label>
              <Input
                value={form.auth_name}
                onChange={(event) =>
                  updateForm({ auth_name: event.target.value })
                }
              />
            </div>
          </div>

          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('Account ID')}</Label>
              <Input
                value={form.account_id}
                onChange={(event) =>
                  updateForm({ account_id: event.target.value })
                }
              />
            </div>
            <div className='space-y-2'>
              <Label>{t('Selected User')}</Label>
              <Input value={form.username || '-'} readOnly />
            </div>
          </div>

          <div className='space-y-2'>
            <Label>{t('Search User')}</Label>
            <div className='relative'>
              <Search className='text-muted-foreground absolute top-2 left-2 size-4' />
              <Input
                value={userKeyword}
                className='pl-8'
                placeholder={t('Search by username')}
                onChange={(event) => setUserKeyword(event.target.value)}
              />
            </div>
            <div className='border-border max-h-40 overflow-y-auto rounded-lg border'>
              {users.length > 0 ? (
                users.map((user) => (
                  <button
                    key={user.id}
                    type='button'
                    className='hover:bg-muted flex w-full items-center justify-between px-3 py-2 text-left text-sm'
                    onClick={() => selectUser(user)}
                  >
                    <span>{user.username}</span>
                    {form.user_id === user.id ? (
                      <Check className='size-4' />
                    ) : null}
                  </button>
                ))
              ) : (
                <div className='text-muted-foreground px-3 py-4 text-center text-sm'>
                  {usersQuery.isLoading ? t('Loading...') : t('No users found')}
                </div>
              )}
            </div>
          </div>

          <div className='space-y-2'>
            <Label>{t('Description')}</Label>
            <Textarea
              value={form.description}
              onChange={(event) =>
                updateForm({ description: event.target.value })
              }
            />
          </div>

          <div className='flex items-center gap-3'>
            <Switch
              checked={form.enabled}
              onCheckedChange={(enabled) => updateForm({ enabled })}
            />
            <Label>{t('Enabled')}</Label>
          </div>
        </div>

        <DialogFooter>
          <Button
            variant='outline'
            type='button'
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={submit}
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
  )
}

function RemoteAuthFilesTable({
  onCreate,
}: {
  onCreate: (authFile: CliproxyAuthFile) => void
}) {
  const { t } = useTranslation()
  const [hasFetched, setHasFetched] = useState(false)
  const query = useQuery({
    queryKey: ['cliproxy-remote-auth-files'],
    queryFn: getCliproxyRemoteAuthFiles,
    enabled: hasFetched,
  })

  const authFiles = query.data?.data ?? []
  const handleFetch = () => {
    if (!hasFetched) {
      setHasFetched(true)
      return
    }
    query.refetch()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Remote Auth Files')}</CardTitle>
        <CardDescription>
          {t('Auth files fetched from the configured Cliproxy API service.')}
        </CardDescription>
        <CardAction>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleFetch}
            disabled={query.isFetching}
          >
            <RefreshCw
              className={query.isFetching ? 'animate-spin' : undefined}
            />
            {t('Refresh')}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        {query.data && !query.data.success ? (
          <Alert variant='destructive' className='mb-4'>
            <AlertDescription>
              {query.data.message || t('Failed to fetch remote auth files')}
            </AlertDescription>
          </Alert>
        ) : null}
        {hasFetched ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Auth Name')}</TableHead>
                <TableHead>{t('Plan')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {authFiles.length > 0 ? (
                authFiles.map((authFile) => (
                  <TableRow key={authFile.authIndex}>
                    <TableCell>{authFile.name || '-'}</TableCell>
                    <TableCell>
                      <RemoteAuthFilePlanCell authFile={authFile} />
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={authFile.enabled ? 'default' : 'secondary'}
                      >
                        {authFile.enabled ? t('Enabled') : t('Disabled')}
                      </Badge>
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() => onCreate(authFile)}
                      >
                        <Plus />
                        {t('Bind User')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground py-8 text-center'
                  >
                    {query.isLoading
                      ? t('Loading...')
                      : t('No auth files found')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        ) : null}
      </CardContent>
    </Card>
  )
}

function BindingTable({
  onEdit,
  isAdmin,
}: {
  onEdit: (binding: CliproxyAuthFileBinding) => void
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [username, setUsername] = useState('')
  const [authIndex, setAuthIndex] = useState('')
  const [deleteTarget, setDeleteTarget] =
    useState<CliproxyAuthFileBinding | null>(null)
  const [refreshingAll, setRefreshingAll] = useState(false)

  const query = useQuery({
    queryKey: ['cliproxy-auth-file-bindings', username, authIndex],
    queryFn: () =>
      getCliproxyAuthFileBindings({
        p: 1,
        page_size: 50,
        username: username.trim() || undefined,
        auth_index: authIndex.trim() || undefined,
      }),
  })

  const refreshMutation = useMutation({
    mutationFn: refreshCliproxyAuthFileBindingUsage,
    onSuccess: (data) => {
      const errorMessage = getApiErrorMessage(
        data,
        t('Failed to refresh usage')
      )
      if (errorMessage) {
        toast.error(errorMessage)
        return
      }
      if (data.data?.last_error) {
        toast.error(data.data.last_error)
      } else {
        toast.success(t('Usage refreshed successfully'))
      }
      queryClient.invalidateQueries({
        queryKey: ['cliproxy-auth-file-bindings'],
      })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to refresh usage'))
    },
  })

  const refreshAll = async () => {
    const enabledBindings = bindings.filter((binding) => binding.enabled)
    if (enabledBindings.length === 0) {
      toast.error(t('No bindings found'))
      return
    }

    setRefreshingAll(true)
    try {
      const summary = await refreshCliproxyAuthFileBindingsUsageAll(
        bindings,
        refreshCliproxyAuthFileBindingUsage
      )
      if (summary.failed === 0) {
        toast.success(t('Refresh succeeded'))
      } else {
        toast.error(
          t('Refresh completed: {{success}} succeeded, {{fail}} failed', {
            success: summary.success,
            fail: summary.failed,
          })
        )
      }
      queryClient.invalidateQueries({
        queryKey: ['cliproxy-auth-file-bindings'],
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Refresh failed'))
    } finally {
      setRefreshingAll(false)
    }
  }

  const deleteMutation = useMutation({
    mutationFn: deleteCliproxyAuthFileBinding,
    onSuccess: (data) => {
      const errorMessage = getApiErrorMessage(
        data,
        t('Failed to delete binding')
      )
      if (errorMessage) {
        toast.error(errorMessage)
        return
      }
      toast.success(t('Binding deleted successfully'))
      queryClient.invalidateQueries({
        queryKey: ['cliproxy-auth-file-bindings'],
      })
      setDeleteTarget(null)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to delete binding'))
    },
  })

  const bindings = query.data?.data?.items ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Auth File Bindings')}</CardTitle>
        <CardDescription>
          {t(
            'Manage the relationship between Cliproxy auth files and new-api users.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-3 md:grid-cols-[1fr_1fr_auto]'>
          <Input
            value={username}
            placeholder={t('Filter by username')}
            onChange={(event) => setUsername(event.target.value)}
          />
          <Input
            value={authIndex}
            placeholder={t('Filter by auth index')}
            onChange={(event) => setAuthIndex(event.target.value)}
          />
          <Button
            variant='outline'
            onClick={refreshAll}
            disabled={refreshingAll || query.isLoading}
          >
            <RefreshCw
              className={refreshingAll ? 'animate-spin' : undefined}
            />
            {t('Refresh All')}
          </Button>
        </div>

        {query.data && !query.data.success ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {query.data.message || t('Failed to fetch bindings')}
            </AlertDescription>
          </Alert>
        ) : null}

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Type')}</TableHead>
              <TableHead>{t('Email')}</TableHead>
              <TableHead>{t('Usage')}</TableHead>
              <TableHead>{t('Last Refreshed')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {bindings.length > 0 ? (
              bindings.map((binding) => (
                <TableRow key={binding.id}>
                  <TableCell>
                    <TooltipProvider delay={200}>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <div className='max-w-[180px] cursor-default' />
                          }
                        >
                          <div className='truncate font-medium'>
                            {binding.username || '-'}
                          </div>
                          {binding.remark ? (
                            <div className='text-muted-foreground truncate text-xs'>
                              {binding.remark}
                            </div>
                          ) : (
                            <div className='text-muted-foreground text-xs'>
                              ID: {binding.user_id}
                            </div>
                          )}
                        </TooltipTrigger>
                        <TooltipContent
                          side='top'
                          align='start'
                          className='max-w-xs whitespace-pre-wrap break-words'
                        >
                          <div className='space-y-1'>
                            <div>{binding.username || '-'}</div>
                            {binding.remark ? (
                              <div className='text-muted-foreground text-xs'>
                                {t('Remark')}: {binding.remark}
                              </div>
                            ) : null}
                            <div className='text-muted-foreground text-xs'>
                              ID: {binding.user_id}
                            </div>
                          </div>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </TableCell>
                  <TableCell>
                    <AuthFileTypeBadge binding={binding} />
                  </TableCell>
                  <TableCell>
                    <AuthFileEmailCell binding={binding} />
                  </TableCell>
                  <TableCell>
                    <BindingUsageCell
                      binding={binding}
                      labels={{
                        fiveHour: t('5-Hour Window'),
                        weekly: t('Weekly Window'),
                        codexFiveHour: `Codex ${t('5-Hour Window')}`,
                        codexWeekly: `Codex ${t('Weekly Window')}`,
                        reset: t('Reset'),
                        quota: t('Quota:'),
                      }}
                    />
                  </TableCell>
                  <TableCell>
                    {formatTimestampToDate(binding.last_refreshed_at)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={binding.enabled ? 'default' : 'secondary'}>
                      {binding.enabled ? t('Enabled') : t('Disabled')}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-right'>
                    <div className='flex justify-end gap-2'>
                      <Button
                        size='icon-sm'
                        variant='outline'
                        onClick={() => refreshMutation.mutate(binding.id)}
                        disabled={refreshMutation.isPending}
                      >
                        <RefreshCw
                          className={
                            refreshMutation.isPending
                              ? 'animate-spin'
                              : undefined
                          }
                        />
                      </Button>
                      {isAdmin ? (
                        <>
                          <Button
                            size='icon-sm'
                            variant='outline'
                            onClick={() => onEdit(binding)}
                          >
                            <Edit />
                          </Button>
                          <Button
                            size='icon-sm'
                            variant='destructive'
                            onClick={() => setDeleteTarget(binding)}
                          >
                            <Trash2 />
                          </Button>
                        </>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className='text-muted-foreground py-8 text-center'
                >
                  {query.isLoading ? t('Loading...') : t('No bindings found')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </CardContent>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Binding')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Are you sure you want to delete this auth file binding? This action cannot be undone.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
              disabled={deleteMutation.isPending}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}

export function CliproxyAuthFiles() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const [dialogState, setDialogState] = useState<BindingDialogState>({
    open: false,
    mode: 'create',
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Auth Files')}</SectionPageLayout.Title>
      {isAdmin ? (
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            onClick={() =>
              setDialogState({
                open: true,
                mode: 'create',
                authFile: { authIndex: '', name: '', enabled: true },
              })
            }
          >
            <Plus />
            {t('Create Binding')}
          </Button>
        </SectionPageLayout.Actions>
      ) : null}
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {isAdmin ? (
            <>
              <ConfigCard />
              <RemoteAuthFilesTable
                onCreate={(authFile) =>
                  setDialogState({ open: true, mode: 'create', authFile })
                }
              />
            </>
          ) : null}
          <BindingTable
            isAdmin={isAdmin}
            onEdit={(binding) =>
              setDialogState({ open: true, mode: 'edit', binding })
            }
          />
        </div>
        {isAdmin ? (
          <BindingDialog
            state={dialogState}
            onOpenChange={(open) => {
              if (!open) {
                setDialogState({ open: false, mode: 'create' })
              }
            }}
          />
        ) : null}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
