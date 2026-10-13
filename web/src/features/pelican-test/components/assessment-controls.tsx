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

import { ModelSelector } from '@/components/model-group-selector'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import type { ModelOption } from '@/features/playground/types'

export function AssessmentControls(props: {
  model: string
  models: ModelOption[]
  onModelChange: (model: string) => void
  modelAvailable: boolean
  busy: boolean
  currentAvailable: boolean
  selectedCount: number
  assessmentPending: boolean
  onAssessCurrent: () => void
  onAssessSelected: () => void
  onStop: () => void
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Model assessment')}</CardTitle>
        <CardDescription>
          {t(
            'Assessments inspect source code, not the rendered animation. Preview the result to judge visual quality.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-3'>
        <section aria-label={t('Assessment model')}>
          <h3 className='mb-2 text-sm font-medium'>{t('Assessment model')}</h3>
          <ModelSelector
            selectedModel={props.model}
            models={props.models}
            onModelChange={props.onModelChange}
            disabled={props.busy || !props.modelAvailable}
          />
          {props.model && (
            <p
              className='text-muted-foreground mt-2 text-xs break-all sm:hidden'
              aria-live='polite'
            >
              {props.model}
            </p>
          )}
        </section>
        <div className='flex flex-wrap gap-2'>
          <Button
            disabled={
              props.busy || !props.modelAvailable || !props.currentAvailable
            }
            onClick={props.onAssessCurrent}
          >
            {t('Assess current result')}
          </Button>
          <Button
            disabled={
              props.busy || !props.modelAvailable || !props.selectedCount
            }
            onClick={props.onAssessSelected}
            variant='outline'
          >
            {t('Assess selected history ({{count}})', {
              count: props.selectedCount,
            })}
          </Button>
          {props.assessmentPending && (
            <Button onClick={props.onStop} variant='outline'>
              {t('Stop')}
            </Button>
          )}
        </div>
        {props.assessmentPending && (
          <p role='status' className='text-muted-foreground text-sm'>
            {t('Assessing selected results...')}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
