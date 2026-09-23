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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CodeBlock } from '@/components/ai-elements/code-block'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { listTypeSafeAnswers } from '@/features/typesafe-evaluations/lib/answers'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { sendJevEvaluation } from '../../api'
import {
  jevResponseSchema,
  parseJevDraft,
  type JevResponse,
} from '../../lib/structured/jev'
import type { GroupOption, ModelOption } from '../../types'

export function JevPlayground(props: {
  model: string
  group: string
  models: ModelOption[]
  groups: GroupOption[]
  isModelLoading: boolean
  onModelChange: (value: string) => void
  onGroupChange: (value: string) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [stateText, setStateText] = useState('{}')
  const [questionsText, setQuestionsText] = useState('{}')
  const [result, setResult] = useState<JevResponse | null>(null)
  const [error, setError] = useState('')
  const [running, setRunning] = useState(false)
  const activeRequest = useRef<AbortController | null>(null)
  useEffect(
    () => () => {
      activeRequest.current?.abort()
      activeRequest.current = null
    },
    []
  )

  const loadExample = () => {
    setStateText(
      JSON.stringify(
        {
          text: t(
            'The package arrived late, but support quickly resolved the issue.'
          ),
        },
        null,
        2
      )
    )
    setQuestionsText(
      JSON.stringify(
        {
          satisfied: {
            type: 'noul',
            instructions: t('Is the customer satisfied with the support?'),
          },
          sentiment: {
            type: 'choice',
            instructions: t('Classify the overall sentiment.'),
            criteria: { positive: null, neutral: null, negative: null },
          },
          rating: {
            type: 'score',
            instructions: t('Rate the overall customer experience.'),
            criteria: [t('Low'), t('Medium'), t('High')],
          },
        },
        null,
        2
      )
    )
    setError('')
    setResult(null)
  }

  const submit = async () => {
    if (
      activeRequest.current ||
      props.isModelLoading ||
      !props.model ||
      !props.group
    ) {
      return
    }
    const draft = parseJevDraft(stateText, questionsText)
    const errors = {
      state_json: t('State must contain valid JSON.'),
      state_schema: t('State must be a JSON string, object, or array.'),
      questions_json: t('Questions must contain valid JSON.'),
      questions_schema: t(
        'Questions must use noul, choice, or score with valid instructions and criteria.'
      ),
    }
    if (draft.error) {
      setError(errors[draft.error])
      return
    }
    const controller = new AbortController()
    activeRequest.current = controller
    setRunning(true)
    setError('')
    setResult(null)
    try {
      const response = await sendJevEvaluation(
        { model: props.model, group: props.group, ...draft.data },
        controller.signal
      )
      if (controller.signal.aborted) return
      const parsed = jevResponseSchema.safeParse(response)
      if (!parsed.success) throw new Error(t('Invalid structured response'))
      setResult(parsed.data)
    } catch (cause) {
      if (!controller.signal.aborted) {
        setError(getServerErrorMessage(cause, t('Request error occurred')))
      }
    } finally {
      if (activeRequest.current === controller) {
        activeRequest.current = null
        setRunning(false)
      }
    }
  }

  const stop = () => {
    activeRequest.current?.abort()
    activeRequest.current = null
    setRunning(false)
  }
  const answers = listTypeSafeAnswers(result?.answers)

  return (
    <section
      className='flex min-w-0 flex-col gap-4'
      aria-label={t('Jev structured evaluation')}
    >
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='flex flex-col gap-1'>
          <h2 className='text-lg font-semibold'>
            {t('Jev structured evaluation')}
          </h2>
          <p className='text-muted-foreground text-sm'>
            {t('Provide state and named questions to receive typed answers.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          disabled={running}
          onClick={loadExample}
        >
          {t('Load example')}
        </Button>
      </div>
      <div className='grid min-w-0 gap-4 lg:grid-cols-2'>
        <div className='flex min-w-0 flex-col gap-2'>
          <Label htmlFor='jev-state'>{t('State (JSON)')}</Label>
          <JsonCodeEditor
            id='jev-state'
            ariaLabel={t('State (JSON)')}
            value={stateText}
            onChange={setStateText}
            disabled={running}
            heightClassName='h-72 min-h-72 max-h-72'
          />
          <p className='text-muted-foreground text-xs'>
            {t('State accepts a JSON string, object, or array.')}
          </p>
        </div>
        <div className='flex min-w-0 flex-col gap-2'>
          <Label htmlFor='jev-questions'>{t('Questions (JSON)')}</Label>
          <JsonCodeEditor
            id='jev-questions'
            ariaLabel={t('Questions (JSON)')}
            value={questionsText}
            onChange={setQuestionsText}
            disabled={running}
            heightClassName='h-72 min-h-72 max-h-72'
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Use noul for a judgment, choice for named options, and score for ordered levels.'
            )}
          </p>
        </div>
      </div>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <ModelGroupSelector
          selectedModel={props.model}
          models={props.models}
          onModelChange={props.onModelChange}
          selectedGroup={props.group}
          groups={props.groups}
          onGroupChange={props.onGroupChange}
          disabled={running || props.isModelLoading}
        />
        {running ? (
          <Button type='button' variant='secondary' onClick={stop}>
            {t('Stop')}
          </Button>
        ) : (
          <Button
            type='button'
            onClick={() => void submit()}
            disabled={
              props.isModelLoading ||
              !props.model ||
              !props.group ||
              props.models.length === 0
            }
          >
            {t('Run evaluation')}
          </Button>
        )}
      </div>
      {running ? (
        <p role='status' className='text-muted-foreground text-sm'>
          {t('Evaluating...')}
        </p>
      ) : null}
      {error ? (
        <Alert variant='destructive'>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}
      {result ? (
        <section
          className='flex min-w-0 flex-col gap-3'
          aria-label={t('Evaluation result')}
        >
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <h3 className='font-medium'>{t('Evaluation result')}</h3>
            <span className='text-muted-foreground text-xs'>
              {result.model} · {t('Input Tokens')}:{' '}
              {formatNumber(result.usage.input_tokens, locale)} ·{' '}
              {t('Output Tokens')}:{' '}
              {formatNumber(result.usage.output_tokens, locale)}
            </span>
          </div>
          <dl className='grid gap-2 sm:grid-cols-2'>
            {answers.map((answer) => (
              <div
                key={answer.name}
                className='flex min-w-0 flex-col gap-1 rounded-md border p-3'
              >
                <dt className='text-muted-foreground text-xs wrap-anywhere'>
                  {answer.name}
                </dt>
                <dd className='font-medium wrap-anywhere'>{answer.value}</dd>
              </div>
            ))}
          </dl>
          <CodeBlock
            code={JSON.stringify(result, null, 2)}
            language='json'
            title={t('Raw response')}
          />
        </section>
      ) : null}
    </section>
  )
}
