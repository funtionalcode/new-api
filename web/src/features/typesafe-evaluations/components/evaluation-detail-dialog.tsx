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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { getTypeSafeEvaluationDetail } from '../api'
import type { TypeSafeEvaluationDetail, TypeSafeStage } from '../types'

function EvaluationText(props: {
  title: string
  text?: string
  truncated?: boolean
  json?: boolean
}) {
  const { t } = useTranslation()
  let text = props.text
  if (props.json && text) {
    try {
      text = JSON.stringify(JSON.parse(text), null, 2)
    } catch {
      // 保留因日志长度限制而截断的原始文本。
    }
  }
  return (
    <section className='min-w-0 space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <h3 className='text-sm font-medium'>{props.title}</h3>
        {props.text ? (
          <CopyButton value={props.text} size='sm'>
            {t('Copy')}
          </CopyButton>
        ) : null}
      </div>
      {props.truncated ? (
        <p className='text-muted-foreground text-xs'>
          {t(
            'Only part of this body was saved because it exceeded the log limit.'
          )}
        </p>
      ) : null}
      {text !== undefined ? (
        <pre className='max-h-80 overflow-auto rounded-md border p-3 text-xs break-all whitespace-pre-wrap'>
          {text}
        </pre>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {t('No input/output was recorded for this log.')}
        </p>
      )}
    </section>
  )
}

function EvaluationContent({ detail }: { detail: TypeSafeEvaluationDetail }) {
  const { t } = useTranslation()
  let requestText: string | undefined
  let responseText: string | undefined
  let questions: string | undefined
  try {
    const input = JSON.parse(detail.request?.body ?? '')
    if (typeof input?.state?.request === 'string') {
      requestText = input.state.request
    }
    if (typeof input?.state?.response === 'string') {
      responseText = input.state.response
    }
    if (input?.questions && typeof input.questions === 'object') {
      questions = JSON.stringify(input.questions, null, 2)
    }
  } catch {
    // 旧记录或不完整 JSON 降级展示已保存原文。
  }
  return (
    <div className='min-w-0 space-y-3'>
      <p className='text-muted-foreground text-sm'>
        {t('Evaluation model')}:{' '}
        {detail.model || t('Evaluation model not recorded')}
      </p>
      {detail.truncated ? (
        <Alert>
          <AlertDescription>
            {t(
              'The text sent for evaluation was truncated. Only the captured portion is available.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}
      <Tabs defaultValue='content'>
        <TabsList
          aria-label={t('View evaluation details')}
          className='h-auto flex-wrap'
        >
          <TabsTrigger value='content'>{t('Evaluation content')}</TabsTrigger>
          <TabsTrigger value='questions'>
            {t('Evaluation questions')}
          </TabsTrigger>
          <TabsTrigger value='result'>{t('Evaluation result')}</TabsTrigger>
        </TabsList>
        <TabsContent value='content' className='space-y-4'>
          {requestText !== undefined || responseText !== undefined ? (
            <>
              <EvaluationText title={t('User request')} text={requestText} />
              {detail.stage === 'after' ? (
                <EvaluationText
                  title={t('Model response')}
                  text={responseText}
                />
              ) : null}
            </>
          ) : (
            <EvaluationText
              title={t('Input')}
              text={detail.request?.body}
              truncated={detail.request?.truncated}
              json
            />
          )}
        </TabsContent>
        <TabsContent value='questions'>
          <EvaluationText title={t('Evaluation questions')} text={questions} />
        </TabsContent>
        <TabsContent value='result'>
          {!detail.response && detail.answers ? (
            <p className='text-muted-foreground mb-2 text-sm'>
              {t(
                'The original output was not recorded. Saved evaluation answers are shown.'
              )}
            </p>
          ) : null}
          <EvaluationText
            title={t('Evaluation result')}
            text={
              detail.response?.body ??
              (detail.answers
                ? JSON.stringify({ answers: detail.answers })
                : undefined)
            }
            truncated={detail.response?.truncated}
            json
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}

export function EvaluationDetailDialog(props: {
  requestId: string
  stage: TypeSafeStage
  onClose: () => void
}) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['typesafe-evaluation-detail', props.requestId, props.stage],
    queryFn: () => getTypeSafeEvaluationDetail(props.requestId, props.stage),
    retry: false,
    gcTime: 0,
  })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={
        props.stage === 'before'
          ? t('Before evaluation details')
          : t('After evaluation details')
      }
      description={`${t('Request ID')}: ${props.requestId}`}
      descriptionClassName='break-all'
      contentClassName='sm:max-w-3xl'
    >
      {query.isPending ? (
        <p role='status' className='text-muted-foreground text-sm'>
          {t('Loading')}
        </p>
      ) : null}
      {query.isError ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Evaluation details are unavailable or you do not have permission.'
            )}
          </AlertDescription>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            {t('Retry')}
          </Button>
        </Alert>
      ) : null}
      {query.isSuccess ? <EvaluationContent detail={query.data} /> : null}
    </Dialog>
  )
}
