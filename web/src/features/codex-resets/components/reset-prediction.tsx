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
import { useQuery } from '@tanstack/react-query'
import {
  ChevronDown,
  ExternalLink,
  Loader2,
  Sparkles,
  Square,
} from 'lucide-react'
import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { ModelResponse } from '@/components/model-response'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { getUserGroups, getUserModels } from '@/features/playground/api'
import {
  streamModelResponse,
  type ModelStreamResponse,
} from '@/features/playground/lib/stream-model-response'
import { getPlaygroundGenerationMode } from '@/features/playground/lib/task/playground-task-models'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  buildResetForecastInput,
  buildResetForecastRequest,
  type ResetForecastHorizon,
  type ResetForecastInput,
} from '../prediction'
import type { CodexResetsData } from '../types'

type ForecastResult = ModelStreamResponse & {
  model: string
  group: string
  input: ResetForecastInput
  status: 'pending' | 'success' | 'error' | 'cancelled'
  error?: string
}

export function ResetPrediction({ data }: { data: CodexResetsData }) {
  const userId = useAuthStore((state) => state.auth.user?.id)
  if (userId === undefined) return null
  return <ResetPredictionWorkspace key={userId} userId={userId} data={data} />
}

function ResetPredictionWorkspace({
  data,
  userId,
}: {
  data: CodexResetsData
  userId: number
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const horizonId = useId()
  const [selectedGroup, setSelectedGroup] = useState('')
  const [selectedModel, setSelectedModel] = useState('')
  const [horizon, setHorizon] = useState<ResetForecastHorizon>(72)
  const [result, setResult] = useState<ForecastResult | null>(null)
  const activeRequest = useRef<AbortController | null>(null)
  const groupsQuery = useQuery({
    queryKey: ['codex-reset-forecast', userId, 'groups'],
    queryFn: getUserGroups,
    retry: false,
  })
  const groups = groupsQuery.data ?? []
  const group = groups.some((option) => option.value === selectedGroup)
    ? selectedGroup
    : (groups[0]?.value ?? '')
  const modelsQuery = useQuery({
    queryKey: ['codex-reset-forecast', userId, 'models', group],
    queryFn: () => getUserModels(group),
    enabled: Boolean(group),
    retry: false,
  })
  const models = useMemo(
    () =>
      (modelsQuery.data ?? []).filter(
        (option) =>
          getPlaygroundGenerationMode('chat', option.value.toLowerCase()) ===
            'chat' &&
          !/(?:^|[/_-])(?:tts|stt|whisper|dall-e|gpt-image|embedding|rerank|sora|flux|stable-diffusion)(?:$|[-_/.\d])/i.test(
            option.value
          )
      ),
    [modelsQuery.data]
  )
  const model = models.some((option) => option.value === selectedModel)
    ? selectedModel
    : (models[0]?.value ?? '')
  const currentInput = useMemo(
    () => buildResetForecastInput(data, horizon),
    [data, horizon]
  )
  const pending = result?.status === 'pending'
  const canPredict =
    !pending &&
    groupsQuery.isSuccess &&
    modelsQuery.isSuccess &&
    !groupsQuery.isFetching &&
    !modelsQuery.isFetching &&
    Boolean(group && model) &&
    currentInput.announcements.length > 0
  const announcementsChanged =
    result &&
    JSON.stringify(result.input.announcements) !==
      JSON.stringify(currentInput.announcements)
  const statusLabels = {
    pending: t('Generating...'),
    cancelled: t('Stopped'),
    error: t('Failed'),
    success: t('Completed'),
  }

  useEffect(
    () => () => {
      const controller = activeRequest.current
      activeRequest.current = null
      controller?.abort()
    },
    []
  )

  const predict = async () => {
    if (!canPredict || activeRequest.current) return
    const controller = new AbortController()
    activeRequest.current = controller
    const input = buildResetForecastInput(data, horizon)
    setResult({
      model,
      group,
      input,
      content: '',
      reasoningContent: '',
      truncated: false,
      status: 'pending',
    })
    try {
      const response = await streamModelResponse(
        buildResetForecastRequest(
          input,
          model,
          group,
          i18n.resolvedLanguage || i18n.language
        ),
        controller.signal,
        (update) => {
          if (activeRequest.current !== controller) return
          setResult((previous) => previous && { ...previous, ...update })
        }
      )
      if (activeRequest.current !== controller) return
      let errorMessage: string | undefined
      if (response.truncated) {
        errorMessage = t(
          'The forecast was truncated. Partial output is preserved.'
        )
      } else if (!response.content) {
        errorMessage = t('The model returned no forecast. Try another model.')
      }
      setResult(
        (previous) =>
          previous && {
            ...previous,
            ...response,
            status:
              response.truncated || !response.content ? 'error' : 'success',
            error: errorMessage,
          }
      )
    } catch (error: unknown) {
      if (activeRequest.current !== controller) return
      let errorMessage: string | undefined
      if (!controller.signal.aborted) {
        errorMessage =
          error instanceof Error
            ? error.message
            : t('Failed to generate reset forecast')
      }
      setResult(
        (previous) =>
          previous && {
            ...previous,
            status: controller.signal.aborted ? 'cancelled' : 'error',
            error: errorMessage,
          }
      )
    } finally {
      if (activeRequest.current === controller) activeRequest.current = null
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <Sparkles className='size-4' aria-hidden='true' />
          {t('Reset forecast')}
        </CardTitle>
        <CardDescription>
          {t(
            'Analyze recent reset announcements with a model. Forecasts are uncertain, not official notices, and use your account quota. No automatic retries.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='flex flex-wrap items-end gap-4'>
          <section className='space-y-2' aria-label={t('Forecast model')}>
            <h3 className='text-sm font-medium'>{t('Forecast model')}</h3>
            <ModelGroupSelector
              aria-label={t('Forecast model')}
              groups={groups}
              models={models}
              selectedGroup={group}
              selectedModel={model}
              onGroupChange={(value) => {
                setSelectedGroup(value)
                setSelectedModel('')
              }}
              onModelChange={setSelectedModel}
              disabled={
                pending || groupsQuery.isFetching || modelsQuery.isFetching
              }
            />
            {model && (
              <p className='text-muted-foreground text-xs break-all sm:hidden'>
                {model} · {group}
              </p>
            )}
          </section>
          <div className='space-y-2'>
            <Label htmlFor={horizonId}>{t('Forecast window')}</Label>
            <NativeSelect
              id={horizonId}
              value={horizon}
              disabled={pending}
              onChange={(event) =>
                setHorizon(Number(event.target.value) as ResetForecastHorizon)
              }
            >
              <NativeSelectOption value={24}>
                {t('Next 24 hours')}
              </NativeSelectOption>
              <NativeSelectOption value={72}>
                {t('Next 72 hours')}
              </NativeSelectOption>
              <NativeSelectOption value={168}>
                {t('Next 7 days')}
              </NativeSelectOption>
            </NativeSelect>
          </div>
          <Button disabled={!canPredict} onClick={() => void predict()}>
            {pending ? (
              <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            ) : (
              <Sparkles className='size-4' aria-hidden='true' />
            )}
            {t('Generate forecast')}
          </Button>
          {pending && (
            <Button
              variant='outline'
              onClick={() => activeRequest.current?.abort()}
            >
              <Square className='size-4' aria-hidden='true' />
              {t('Stop')}
            </Button>
          )}
        </div>

        {(groupsQuery.isError || modelsQuery.isError) && (
          <ErrorState
            className='min-h-0 py-4'
            title={t('Failed to load forecast models')}
            onRetry={() => {
              if (groupsQuery.isError) void groupsQuery.refetch()
              if (modelsQuery.isError) void modelsQuery.refetch()
            }}
          />
        )}
        {modelsQuery.isSuccess && models.length === 0 && (
          <p role='status' className='text-muted-foreground text-sm'>
            {t('No text generation models are available in this group.')}
          </p>
        )}
        <p className='text-muted-foreground text-xs'>
          {t(
            'Uses up to 20 recent cached reset announcements, not a complete official feed.'
          )}{' '}
          {t('Synced')}:{' '}
          {data.sync.last_success_at
            ? formatTimestampToDate(data.sync.last_success_at)
            : t('Unknown')}
        </p>
        {currentInput.announcements.length === 0 && (
          <p role='status' className='text-muted-foreground text-sm'>
            {t('No announcements are available for a forecast.')}
          </p>
        )}
        {data.sync.last_error && (
          <Alert>
            <AlertDescription>
              {t(
                'The latest announcement sync failed. Forecasts use previously cached data.'
              )}
            </AlertDescription>
          </Alert>
        )}
        {result && (
          <section aria-label={t('Forecast result')} className='space-y-4'>
            <div className='flex flex-wrap items-center gap-2 text-xs'>
              <Badge
                variant={
                  result.status === 'error' ? 'destructive' : 'secondary'
                }
              >
                {statusLabels[result.status]}
              </Badge>
              <span className='break-all'>
                {result.model} · {result.group}
              </span>
              <span>
                {formatTimestampToDate(Date.parse(result.input.as_of) / 1000)}
              </span>
              <span>
                {t('Next {{hours}} hours', {
                  hours: formatNumber(result.input.horizon_hours, locale),
                })}
              </span>
            </div>
            {announcementsChanged && (
              <Alert>
                <AlertDescription>
                  {t(
                    'Announcements have changed since this forecast. Generate a new forecast for updated evidence.'
                  )}
                </AlertDescription>
              </Alert>
            )}
            {result.error && (
              <Alert variant='destructive'>
                <AlertDescription>{result.error}</AlertDescription>
              </Alert>
            )}
            <ModelResponse
              content={result.content}
              reasoningContent={result.reasoningContent}
              pending={pending}
            />
            <Collapsible>
              <CollapsibleTrigger
                render={<Button variant='outline' size='sm' />}
              >
                {t('Announcement evidence ({{total}})', {
                  total: formatNumber(
                    result.input.announcements.length,
                    locale
                  ),
                })}
                <ChevronDown className='size-4' aria-hidden='true' />
              </CollapsibleTrigger>
              <CollapsibleContent className='pt-3'>
                <p className='text-muted-foreground mb-3 text-xs'>
                  {t('Synced')}:{' '}
                  {result.input.last_successful_sync_at
                    ? formatTimestampToDate(
                        Date.parse(result.input.last_successful_sync_at) / 1000
                      )
                    : t('Unknown')}
                </p>
                <ol className='max-h-72 space-y-3 overflow-auto text-sm'>
                  {result.input.announcements.map((announcement) => (
                    <li key={announcement.announcement} className='space-y-1'>
                      <div className='flex flex-wrap items-center gap-2'>
                        <span>
                          [{formatNumber(announcement.announcement, locale)}]
                        </span>
                        <span className='text-muted-foreground text-xs'>
                          {formatTimestampToDate(
                            Date.parse(announcement.announced_at) / 1000
                          )}
                        </span>
                        {announcement.url && (
                          <Button
                            variant='link'
                            size='sm'
                            render={
                              <a
                                href={announcement.url}
                                target='_blank'
                                rel='noopener noreferrer'
                              />
                            }
                          >
                            {t('Source')}
                            <ExternalLink
                              className='size-3'
                              aria-hidden='true'
                            />
                          </Button>
                        )}
                      </div>
                      <p className='break-words whitespace-pre-wrap'>
                        {announcement.text}
                      </p>
                      {announcement.text_truncated && (
                        <p className='text-muted-foreground text-xs'>
                          {t('Announcement text was shortened for analysis.')}
                        </p>
                      )}
                    </li>
                  ))}
                </ol>
              </CollapsibleContent>
            </Collapsible>
          </section>
        )}
      </CardContent>
    </Card>
  )
}
