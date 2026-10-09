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
import type { ComponentProps } from 'react'
import { useTranslation } from 'react-i18next'

import { requireServerSuccess } from '@/lib/server-error-message'

import { searchChannels } from '../api'
import { ModelMappingEditor } from './model-mapping-editor'

export function ChannelModelMappingEditor(
  props: ComponentProps<typeof ModelMappingEditor> & {
    currentChannelId?: number
  }
) {
  const { t } = useTranslation()
  const channels = useQuery({
    queryKey: ['channels', 'model-mapping-targets'],
    queryFn: async () => {
      const channels: { id: number; name: string; models: string[] }[] = []
      for (let page = 1; ; page++) {
        const data = requireServerSuccess(
          await searchChannels({ p: page, page_size: 100, status: 'enabled' })
        ).data
        const items = data?.items ?? []
        channels.push(
          ...items.map((item) => ({
            id: item.id,
            name: item.name,
            models: (item.models ?? '')
              .split(',')
              .map((model) => model.trim())
              .filter(Boolean),
          }))
        )
        if (items.length < 100 || page * 100 >= (data?.total ?? 0)) break
      }
      return channels
    },
    enabled: !props.disabled,
    staleTime: 60000,
  })
  return (
    <div className='space-y-2'>
      <ModelMappingEditor
        {...props}
        channelOptions={(channels.data ?? []).filter(
          (channel) => channel.id !== props.currentChannelId
        )}
      />
      <p className='text-muted-foreground text-xs'>
        {t(
          'When a target channel is selected, use a model exposed by that channel. Its own model mapping still applies.'
        )}
      </p>
    </div>
  )
}
