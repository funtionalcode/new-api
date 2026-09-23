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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'

import type { TypeSafeEvaluationResult } from '../types'
import { DetailRow, DetailSection } from './dialogs/log-detail-layout'

export function AdaptiveReasoningDetails(props: {
  results?: TypeSafeEvaluationResult[]
}) {
  const { t } = useTranslation()
  let decision: TypeSafeEvaluationResult | undefined
  for (const item of props.results ?? []) {
    if (item.stage === 'adaptive_reasoning' && !item.parent_request_id) {
      decision = item
    }
  }
  if (!decision) return null
  const answers = decision.answers
  let source = t('New evaluation')
  if (decision.source === 'cache') source = t('Cached')
  else if (decision.source === 'request_retry') source = t('Retry')
  let reason = decision.reason
  if (reason === 'channel_unavailable_or_forbidden') {
    reason = t(
      'The evaluation channel is unavailable or you do not have access. No evaluation was run.'
    )
  } else if (
    reason === 'model_not_allowed' ||
    reason === 'model_or_group_not_allowed'
  ) {
    reason = t(
      'The evaluation model is not allowed for this user, token, or group.'
    )
  } else if (reason === 'timeout_or_cancelled') {
    reason = t('Evaluation timed out or was cancelled.')
  } else if (reason === 'insufficient_context') {
    reason = t(
      'No task context was available. The original reasoning effort was kept.'
    )
  }
  let status = t('Error')
  if (decision.status === 'success') status = t('Success')
  else if (decision.status === 'skipped') status = t('Skipped')
  return (
    <DetailSection label={t('Jev reasoning evaluation')}>
      <div className='space-y-1.5 py-1'>
        <div className='flex flex-wrap items-center gap-2'>
          <Badge
            variant={decision.status === 'error' ? 'destructive' : 'outline'}
          >
            {status}
          </Badge>
          <Badge variant={decision.applied ? 'default' : 'outline'}>
            {decision.applied ? t('Applied') : t('Original effort kept')}
          </Badge>
          {decision.truncated && (
            <Badge variant='outline'>{t('Evaluation content truncated')}</Badge>
          )}
        </div>
        <DetailRow
          label={t('Evaluation Model')}
          value={decision.model || '-'}
          mono
        />
        {decision.effort && (
          <DetailRow
            label={t('Reasoning Effort')}
            value={decision.effort}
            mono
          />
        )}
        {decision.requested_effort && (
          <DetailRow
            label={t('Original effort')}
            value={decision.requested_effort}
            mono
          />
        )}
        {decision.generations !== undefined && (
          <DetailRow
            label={t('Effective reuse (generations)')}
            value={decision.generations}
          />
        )}
        {decision.remaining !== undefined && (
          <DetailRow
            label={t('Remaining reuse (generations)')}
            value={decision.remaining}
          />
        )}
        {decision.source && <DetailRow label={t('Source')} value={source} />}
        {reason && <DetailRow label={t('Reason')} value={reason} />}
        {answers && Object.keys(answers).length > 0 && (
          <details className='mt-2 min-w-0 rounded-md border'>
            <summary className='cursor-pointer px-2 py-1.5 text-xs font-medium'>
              {t('View evaluation details')}
            </summary>
            <div className='space-y-1 px-2 pb-2'>
              <div className='flex justify-end'>
                <CopyButton value={JSON.stringify(answers, null, 2)} size='sm'>
                  {t('Copy')}
                </CopyButton>
              </div>
              <pre className='max-h-64 overflow-auto text-xs break-all whitespace-pre-wrap'>
                {JSON.stringify(answers, null, 2)}
              </pre>
            </div>
          </details>
        )}
      </div>
    </DetailSection>
  )
}
