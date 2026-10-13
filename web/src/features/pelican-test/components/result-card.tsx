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
import { RotateCcw, ThumbsDown, ThumbsUp } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  WebPreview,
  WebPreviewBody,
} from '@/components/ai-elements/web-preview'
import { CopyButton } from '@/components/copy-button'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import {
  buildPreviewDocument,
  hasSvgMarkup,
  type PelicanTestRating,
  type PelicanTestRecord,
} from '../lib'

export function ResultCard(props: {
  record: PelicanTestRecord
  onRatingChange: (rating: PelicanTestRating) => void
  onRerun?: () => void
  rerunDisabled?: boolean
}) {
  const { t, i18n } = useTranslation()
  const [previewEnabled, setPreviewEnabled] = useState(false)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const preview = useMemo(
    () => buildPreviewDocument(props.record.content),
    [props.record.content]
  )
  const hasSvg = hasSvgMarkup(props.record.content)
  const createdAt = new Intl.DateTimeFormat(locale, {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(props.record.createdAt)
  let statusLabel = t('Completed')
  if (props.record.status === 'error') statusLabel = t('Failed')
  if (props.record.status === 'running') statusLabel = t('Generating...')
  if (props.record.status === 'cancelled') {
    statusLabel = t('Generation was interrupted')
  }

  return (
    <Card
      role='region'
      className='min-w-0'
      aria-label={t('Result for {{model}}', { model: props.record.model })}
    >
      <CardHeader>
        <div className='flex flex-wrap items-start justify-between gap-2'>
          <div className='min-w-0'>
            <CardTitle className='break-all'>{props.record.model}</CardTitle>
            <CardDescription>
              {props.record.group} · {createdAt}
            </CardDescription>
          </div>
          <Badge
            variant={
              props.record.status === 'error' ? 'destructive' : 'outline'
            }
          >
            {statusLabel}
          </Badge>
        </div>
        {props.record.durationMs !== undefined && (
          <CardDescription>
            {t('Duration: {{seconds}} s', {
              seconds: formatNumber(props.record.durationMs / 1000, locale),
            })}
          </CardDescription>
        )}
      </CardHeader>
      <CardContent className='space-y-3'>
        {props.record.errorMessage && (
          <Alert variant='destructive'>
            <AlertDescription>{props.record.errorMessage}</AlertDescription>
          </Alert>
        )}
        {props.record.status === 'success' && (
          <>
            <Badge variant={hasSvg ? 'outline' : 'warning'}>
              {hasSvg ? t('SVG detected') : t('SVG not detected')}
            </Badge>
            <Tabs defaultValue='preview'>
              <TabsList>
                <TabsTrigger value='preview'>{t('Preview')}</TabsTrigger>
                <TabsTrigger value='source'>
                  {t('View generated HTML')}
                </TabsTrigger>
              </TabsList>
              <TabsContent value='preview' className='min-w-0'>
                {previewEnabled ? (
                  <WebPreview className='h-80 overflow-hidden bg-white'>
                    <WebPreviewBody
                      srcDoc={preview}
                      sandbox='allow-scripts'
                      referrerPolicy='no-referrer'
                      title={t('Pelican HTML preview')}
                      className='h-80'
                    />
                  </WebPreview>
                ) : (
                  <div className='bg-muted/20 flex h-80 flex-col items-center justify-center gap-3 rounded-lg border px-4 text-center'>
                    <p className='text-muted-foreground text-sm'>
                      {t(
                        'Preview runs generated code in an isolated frame with network access blocked.'
                      )}
                    </p>
                    <Button
                      onClick={() => setPreviewEnabled(true)}
                      variant='outline'
                    >
                      {t('Load preview')}
                    </Button>
                  </div>
                )}
                {previewEnabled && (
                  <Button
                    className='mt-2'
                    size='sm'
                    variant='ghost'
                    onClick={() => setPreviewEnabled(false)}
                  >
                    {t('Stop preview')}
                  </Button>
                )}
              </TabsContent>
              <TabsContent value='source'>
                <CopyButton
                  value={props.record.content}
                  size='sm'
                  variant='outline'
                >
                  {t('Copy')}
                </CopyButton>
                <pre className='bg-muted/30 mt-2 max-h-80 overflow-auto rounded-lg border p-3 text-xs leading-5 break-all whitespace-pre-wrap'>
                  {props.record.content}
                </pre>
              </TabsContent>
            </Tabs>
            <ToggleGroup
              aria-label={t('How is this result?')}
              value={props.record.rating ? [props.record.rating] : []}
              onValueChange={(values) => {
                if (values[0] === 'good' || values[0] === 'needs_review') {
                  props.onRatingChange(values[0])
                }
              }}
              variant='outline'
              className='max-w-full flex-wrap'
              spacing={1}
            >
              <ToggleGroupItem value='good'>
                <ThumbsUp />
                {t('Good result')}
              </ToggleGroupItem>
              <ToggleGroupItem value='needs_review'>
                <ThumbsDown />
                {t('Needs review')}
              </ToggleGroupItem>
            </ToggleGroup>
          </>
        )}
        {props.record.status === 'error' && props.record.content && (
          <pre className='max-h-40 overflow-auto text-xs whitespace-pre-wrap'>
            {props.record.content}
          </pre>
        )}
        {props.onRerun && (
          <Button
            onClick={props.onRerun}
            disabled={props.rerunDisabled}
            size='sm'
            variant='outline'
          >
            <RotateCcw />
            {t('Test again with selected model')}
          </Button>
        )}
        {props.record.assessments.length > 0 && (
          <section className='space-y-2' aria-label={t('Assessment history')}>
            <h3 className='text-sm font-semibold'>{t('Assessment history')}</h3>
            {props.record.assessments.map((assessment) => (
              <details key={assessment.id} className='rounded-lg border p-3'>
                <summary className='cursor-pointer text-sm'>
                  {assessment.model} ·{' '}
                  {new Intl.DateTimeFormat(locale, {
                    dateStyle: 'short',
                    timeStyle: 'short',
                  }).format(assessment.createdAt)}
                  {assessment.status === 'error' && (
                    <Badge className='ml-2' variant='destructive'>
                      {t('Failed')}
                    </Badge>
                  )}
                </summary>
                <p className='mt-3 text-sm leading-6 break-words whitespace-pre-wrap'>
                  {assessment.content}
                </p>
              </details>
            ))}
          </section>
        )}
      </CardContent>
    </Card>
  )
}
