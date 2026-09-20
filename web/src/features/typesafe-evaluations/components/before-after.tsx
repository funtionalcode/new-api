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
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import { listTypeSafeAnswers } from '../lib/answers'
import type { TypeSafeStageSummary } from '../types'

function statusVariant(
  status?: string
): 'default' | 'destructive' | 'warning' | 'outline' {
  if (status === 'success') return 'default'
  if (status === 'skipped') return 'warning'
  if (status === 'error') return 'destructive'
  return 'outline'
}

function statusLabel(status: string | undefined, t: TFunction) {
  if (status === 'success') return t('Success')
  if (status === 'skipped') return t('Skipped')
  if (status === 'error') return t('Error')
  if (!status) return t('No answers')
  return status
}

function TypeSafeStagePanel(props: {
  title: string
  stage?: TypeSafeStageSummary
}) {
  const { t } = useTranslation()
  const answers = listTypeSafeAnswers(props.stage?.answers)

  return (
    <section className='bg-background min-w-0 rounded-lg border p-3'>
      <div className='mb-2 flex flex-wrap items-center gap-2'>
        <h3 className='text-sm font-medium'>{props.title}</h3>
        {props.stage?.status ? (
          <Badge variant={statusVariant(props.stage.status)}>
            {statusLabel(props.stage.status, t)}
          </Badge>
        ) : null}
        {props.stage?.truncated ? (
          <Badge variant='outline'>{t('Truncated')}</Badge>
        ) : null}
      </div>
      {answers.length > 0 ? (
        <dl className='space-y-1.5'>
          {answers.map((answer) => (
            <div
              key={answer.name}
              className='flex items-start justify-between gap-3'
            >
              <dt className='text-muted-foreground min-w-0 truncate text-xs'>
                {answer.name}
              </dt>
              <dd className='text-right text-sm font-medium wrap-break-word'>
                {answer.value}
              </dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {props.stage?.reason
            ? `${t('Reason')}: ${props.stage.reason}`
            : t('No answers')}
        </p>
      )}
      {answers.length > 0 && props.stage?.reason ? (
        <p className='text-muted-foreground mt-2 text-xs'>
          {t('Reason')}: {props.stage.reason}
        </p>
      ) : null}
    </section>
  )
}

export function TypeSafeBeforeAfter(props: {
  before?: TypeSafeStageSummary
  after?: TypeSafeStageSummary
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <div className={cn('grid gap-3 md:grid-cols-2', props.className)}>
      <TypeSafeStagePanel title={t('Before')} stage={props.before} />
      <TypeSafeStagePanel title={t('After')} stage={props.after} />
    </div>
  )
}
