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
import { Brain, ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from '@/components/ai-elements/reasoning'
import { CopyButton } from '@/components/copy-button'
import { cn } from '@/lib/utils'

export function ModelResponse(props: {
  content: string
  reasoningContent?: string
  pending?: boolean
}) {
  const { t } = useTranslation()
  const [reasoningOpen, setReasoningOpen] = useState(true)

  return (
    <section className='min-w-0 space-y-4' aria-label={t('Model response')}>
      {props.reasoningContent ? (
        <Reasoning
          className='mb-0 min-w-0'
          defaultOpen={false}
          open={reasoningOpen}
          onOpenChange={setReasoningOpen}
        >
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <ReasoningTrigger>
              <Brain className='size-3.5' aria-hidden='true' />
              <span>{t('Reasoning process')}</span>
              <ChevronDown
                className={cn('size-3.5', reasoningOpen && 'rotate-180')}
                aria-hidden='true'
              />
            </ReasoningTrigger>
            <CopyButton
              value={props.reasoningContent}
              aria-label={t('Copy reasoning content')}
              size='sm'
              variant='outline'
            >
              {t('Copy')}
            </CopyButton>
          </div>
          <ReasoningContent className='max-h-64 overflow-auto break-words [&_pre]:break-all [&_pre]:whitespace-pre-wrap'>
            {props.reasoningContent}
          </ReasoningContent>
        </Reasoning>
      ) : (
        <div className='space-y-2'>
          <h3 className='text-sm font-semibold'>{t('Reasoning process')}</h3>
          <p className='text-muted-foreground text-sm'>
            {props.pending
              ? t('Waiting for the model response')
              : t('No reasoning content is available')}
          </p>
        </div>
      )}
      <div className='min-w-0 space-y-2'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <h3 className='text-sm font-semibold'>{t('Output')}</h3>
          {props.content && (
            <CopyButton
              value={props.content}
              aria-label={t('Copy output content')}
              size='sm'
              variant='outline'
            >
              {t('Copy')}
            </CopyButton>
          )}
        </div>
        {props.content ? (
          <pre
            aria-label={t('Output content')}
            className='bg-muted/30 max-h-80 overflow-auto rounded-lg border p-3 text-xs leading-5 break-all whitespace-pre-wrap'
          >
            {props.content}
          </pre>
        ) : (
          <p className='text-muted-foreground text-sm'>
            {props.pending
              ? t('Waiting for the model response')
              : t('No output content is available')}
          </p>
        )}
      </div>
    </section>
  )
}
