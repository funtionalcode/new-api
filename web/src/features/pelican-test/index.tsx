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
import { Bird, Play, Square } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { getUserGroups, getUserModels } from '@/features/playground/api'
import { getPlaygroundGenerationMode } from '@/features/playground/lib/task/playground-task-models'
import { useAuthStore } from '@/stores/auth-store'

import { AssessmentControls } from './components/assessment-controls'
import { ResultCard } from './components/result-card'
import { TestHistory } from './components/test-history'
import { PELICAN_TEST_PROMPT } from './lib'
import { usePelicanTest } from './use-pelican-test'

export function PelicanTest() {
  const userId = useAuthStore((state) => state.auth.user?.id)
  if (userId === undefined) return null
  return <PelicanTestWorkspace key={userId} userId={userId} />
}

function PelicanTestWorkspace(props: { userId: number }) {
  const { t } = useTranslation()
  const test = usePelicanTest(props.userId)
  const [group, setGroup] = useState('')
  const [model, setModel] = useState('')
  const [reviewModel, setReviewModel] = useState('')
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const groupsQuery = useQuery({
    queryKey: ['pelican-test', props.userId, 'groups'],
    queryFn: getUserGroups,
  })
  const modelsQuery = useQuery({
    queryKey: ['pelican-test', props.userId, 'models', group],
    queryFn: () => getUserModels(group),
    enabled: Boolean(group),
  })
  const models = useMemo(
    () =>
      (modelsQuery.data ?? []).filter(
        (option) => getPlaygroundGenerationMode('chat', option.value) === 'chat'
      ),
    [modelsQuery.data]
  )
  const groups = groupsQuery.data ?? []

  useEffect(() => {
    if (
      groupsQuery.data &&
      !groupsQuery.data.some((option) => option.value === group)
    ) {
      setGroup(groupsQuery.data[0]?.value ?? '')
    }
  }, [group, groupsQuery.data])

  useEffect(() => {
    if (!modelsQuery.isSuccess) return
    if (!models.some((option) => option.value === model)) {
      setModel(models[0]?.value ?? '')
    }
    if (!models.some((option) => option.value === reviewModel)) {
      setReviewModel(models[0]?.value ?? '')
    }
  }, [model, reviewModel, models, modelsQuery.isSuccess])

  const current = test.history.find(
    (record) => record.id === test.currentRecordId
  )
  const selected = test.history.filter((record) =>
    selectedIds.includes(record.id)
  )
  const selectableModels =
    modelsQuery.isSuccess &&
    !modelsQuery.isFetching &&
    !groupsQuery.isError &&
    groups.some((option) => option.value === group)
  const canGenerate =
    test.ready &&
    selectableModels &&
    models.some((option) => option.value === model)
  const busy = test.generation.isPending || test.assessment.isPending
  const rerun = () => test.generation.mutate({ model, group })
  const toggleSelection = (id: string) =>
    setSelectedIds((previous) => {
      if (previous.includes(id)) return previous.filter((value) => value !== id)
      return previous.length < 4 ? [...previous, id] : previous
    })

  return (
    <div className='min-h-0 flex-1 overflow-y-auto'>
      <div className='mx-auto flex w-full max-w-7xl flex-col gap-5 px-4 py-6 md:px-6'>
        <header className='flex items-start gap-3'>
          <Bird
            className='text-primary mt-1 size-6 shrink-0'
            aria-hidden='true'
          />
          <div>
            <h1 className='text-xl font-semibold tracking-tight'>
              {t('Pelican Test')}
            </h1>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Compare how different models generate the same Pelican SVG animation.'
              )}
            </p>
          </div>
        </header>

        <Card>
          <CardHeader>
            <CardTitle>{t('Run a Pelican test')}</CardTitle>
            <CardDescription>
              {t(
                'Generation and model assessments use your account quota. Requests are not automatically retried.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            <section aria-label={t('Generation model')}>
              <h2 className='mb-2 text-sm font-medium'>
                {t('Generation model')}
              </h2>
              <ModelGroupSelector
                disabled={busy || groupsQuery.isFetching || groups.length === 0}
                groups={groups}
                models={models}
                onGroupChange={(value) => {
                  setGroup(value)
                  setModel('')
                  setReviewModel('')
                }}
                onModelChange={setModel}
                selectedGroup={group}
                selectedModel={model}
              />
              {model && (
                <p
                  className='text-muted-foreground mt-2 text-xs break-all sm:hidden'
                  aria-live='polite'
                >
                  {model} · {group}
                </p>
              )}
            </section>
            {(modelsQuery.isError || groupsQuery.isError) && (
              <ErrorState
                title={t('Failed to load models')}
                className='min-h-24'
                onRetry={() => {
                  void groupsQuery.refetch()
                  if (group) void modelsQuery.refetch()
                }}
              />
            )}
            <div className='bg-muted/40 rounded-lg border p-3'>
              <h2 className='mb-1 text-xs font-medium'>
                {t('Fixed test prompt')}
              </h2>
              <p className='text-sm leading-6'>{PELICAN_TEST_PROMPT}</p>
            </div>
            <div className='flex flex-wrap items-center gap-3'>
              {(modelsQuery.isFetching || groupsQuery.isFetching) && (
                <span className='text-muted-foreground text-sm' role='status'>
                  {t('Loading models...')}
                </span>
              )}
              <Button disabled={!canGenerate || busy} onClick={rerun}>
                <Play />
                {test.generation.isPending
                  ? t('Generating...')
                  : t('Generate and save result')}
              </Button>
              {test.generation.isPending && (
                <Button variant='outline' onClick={test.stopGeneration}>
                  <Square />
                  {t('Stop')}
                </Button>
              )}
              {!model && modelsQuery.isSuccess && (
                <span className='text-muted-foreground text-sm'>
                  {t('No model found.')}
                </span>
              )}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t(
                'History is saved only in this browser for the current user, not synchronized between devices.'
              )}
            </p>
          </CardContent>
        </Card>

        {test.storageError && (
          <Alert variant='destructive'>
            <AlertDescription>{t(test.storageError)}</AlertDescription>
          </Alert>
        )}
        {current && (
          <ResultCard
            key={current.id}
            record={current}
            onRatingChange={(rating) => test.rate(current.id, rating)}
            rerunDisabled={!canGenerate || busy}
            onRerun={rerun}
          />
        )}

        <AssessmentControls
          model={reviewModel}
          onModelChange={setReviewModel}
          models={models}
          busy={busy}
          modelAvailable={Boolean(
            selectableModels &&
            models.some((option) => option.value === reviewModel)
          )}
          selectedCount={
            selected.filter((record) => record.status === 'success').length
          }
          currentAvailable={current?.status === 'success'}
          assessmentPending={test.assessment.isPending}
          onStop={test.stopAssessment}
          onAssessCurrent={() => {
            if (current) {
              test.assessment.mutate({
                model: reviewModel,
                group,
                ids: [current.id],
              })
            }
          }}
          onAssessSelected={() =>
            test.assessment.mutate({
              model: reviewModel,
              group,
              ids: selectedIds,
            })
          }
        />

        <TestHistory
          records={test.history}
          currentId={test.currentRecordId}
          selectedIds={selectedIds}
          onSelect={toggleSelection}
          onView={test.setCurrentRecordId}
        />

        {selected.length > 1 && (
          <section
            aria-label={t('Compare selected results')}
            className='space-y-3'
          >
            <h2 className='text-lg font-semibold'>
              {t('Compare selected results')}
            </h2>
            <div className='grid min-w-0 gap-4 xl:grid-cols-2'>
              {selected.map((record) => (
                <ResultCard
                  key={record.id}
                  record={record}
                  onRatingChange={(rating) => test.rate(record.id, rating)}
                />
              ))}
            </div>
          </section>
        )}
      </div>
    </div>
  )
}
