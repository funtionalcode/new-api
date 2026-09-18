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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { TypeSafeExchange, TypeSafeLogBody } from '../types'

function TypeSafeBody(props: { payload?: TypeSafeLogBody }) {
  const { t } = useTranslation()
  if (!props.payload) {
    return (
      <p className='text-muted-foreground text-xs'>
        {t('No input/output was recorded for this log.')}
      </p>
    )
  }

  let preview = props.payload.body
  try {
    preview = JSON.stringify(JSON.parse(preview), null, 2)
  } catch {
    // 截断后的 JSON 仍按原始文本展示。
  }

  return (
    <div className='flex min-w-0 flex-col gap-2'>
      <div className='flex items-center justify-end gap-2'>
        {props.payload.truncated && (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Only part of this body was saved because it exceeded the log limit.'
            )}
          </p>
        )}
        <CopyButton value={props.payload.body} size='sm'>
          {t('Copy')}
        </CopyButton>
      </div>
      <pre className='max-h-80 overflow-auto rounded-md border p-3 text-xs break-all whitespace-pre-wrap'>
        {preview}
      </pre>
    </div>
  )
}

export function TypeSafeIODetails(props: { exchange?: TypeSafeExchange }) {
  const { t } = useTranslation()
  return (
    <details className='min-w-0 rounded-md border'>
      <summary className='cursor-pointer px-3 py-2 text-sm font-medium'>
        {t('TypeSafe input and output')}
      </summary>
      <div className='px-3 pb-3'>
        {props.exchange ? (
          <Tabs defaultValue='input'>
            <TabsList aria-label={t('TypeSafe input and output')}>
              <TabsTrigger value='input'>{t('Input')}</TabsTrigger>
              <TabsTrigger value='output'>{t('Output')}</TabsTrigger>
            </TabsList>
            <TabsContent value='input'>
              <TypeSafeBody payload={props.exchange.request} />
            </TabsContent>
            <TabsContent value='output'>
              <TypeSafeBody payload={props.exchange.response} />
            </TabsContent>
          </Tabs>
        ) : (
          <p className='text-muted-foreground text-xs'>
            {t('No input/output was recorded for this log.')}
          </p>
        )}
      </div>
    </details>
  )
}
