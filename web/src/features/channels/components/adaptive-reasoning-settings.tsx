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
import { useFormContext, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { requireServerSuccess } from '@/lib/server-error-message'

import { searchChannels } from '../api'
import {
  ADAPTIVE_REASONING_DEFAULTS,
  supportsAdaptiveReasoning,
} from '../lib/adaptive-reasoning'
import type { ChannelFormValues } from '../lib/channel-form'

const efforts = [
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
].map((value) => ({ value, label: value }))
const windows = [1, 2, 5, 10].map((value) => ({
  value: String(value),
  label: String(value),
}))

export function AdaptiveReasoningSettings(props: {
  channelType: number
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const form = useFormContext<ChannelFormValues>()
  const config =
    useWatch({ control: form.control, name: 'adaptive_reasoning' }) ??
    ADAPTIVE_REASONING_DEFAULTS
  const supported = supportsAdaptiveReasoning(props.channelType)
  const channels = useQuery({
    queryKey: ['channels', 'adaptive-reasoning-typesafe'],
    queryFn: async () =>
      requireServerSuccess(
        await searchChannels({ type: 66, p: 1, page_size: 100 })
      ).data?.items ?? [],
    enabled: supported && config.enabled && !props.disabled,
    staleTime: 60000,
  })
  if (!supported) return null
  const options = (channels.data ?? [])
    .filter((channel) => channel.status === 1)
    .map((channel) => ({
      value: String(channel.id),
      label: `${channel.name} (#${channel.id})`,
    }))
  if (
    config.channel_id > 0 &&
    !options.some((option) => option.value === String(config.channel_id))
  ) {
    options.push({
      value: String(config.channel_id),
      label: `#${config.channel_id}`,
    })
  }

  return (
    <div
      className='space-y-4 rounded-md border p-4'
      role='group'
      aria-label={t('Adaptive reasoning')}
    >
      <FormField
        control={form.control}
        name='adaptive_reasoning.enabled'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between gap-4'>
            <div className='space-y-1'>
              <FormLabel>{t('Enable adaptive reasoning')}</FormLabel>
              <FormDescription>
                {t(
                  'Jev selects reasoning effort before each generation. Supports Claude Messages, Chat Completions and Responses, including WebSocket.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={config.enabled}
                disabled={props.disabled}
                onCheckedChange={(enabled) =>
                  form.setValue(
                    'adaptive_reasoning',
                    { ...config, enabled },
                    { shouldDirty: true, shouldValidate: true }
                  )
                }
                onBlur={field.onBlur}
                ref={field.ref}
              />
            </FormControl>
          </FormItem>
        )}
      />
      {config.enabled && props.channelType === 14 && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'Claude uses native effort levels supported by the selected model. Unsupported models keep their original settings.'
          )}
        </p>
      )}
      {config.enabled && (
        <fieldset
          disabled={props.disabled}
          className='grid gap-4 sm:grid-cols-2'
        >
          <FormField
            control={form.control}
            name='adaptive_reasoning.channel_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('TypeSafe channel')}</FormLabel>
                <Select
                  items={options}
                  value={(field.value ?? 0) > 0 ? String(field.value) : ''}
                  onValueChange={(value) => field.onChange(Number(value))}
                  disabled={props.disabled || channels.isPending}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue
                        placeholder={t('Select a TypeSafe channel')}
                      />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {options.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='adaptive_reasoning.model'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Evaluation Model')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='jev-latest'
                    disabled={props.disabled}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='adaptive_reasoning.efforts'
            render={({ field }) => (
              <FormItem className='sm:col-span-2'>
                <FormLabel>{t('Allowed reasoning efforts')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    options={efforts}
                    selected={field.value ?? config.efforts}
                    onChange={field.onChange}
                    disabled={props.disabled}
                    placeholder={t('Select at least one reasoning effort')}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Choose only effort levels supported by the upstream model. The selected effort overrides client and channel effort settings.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='adaptive_reasoning.max_reuse_generations'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Maximum reuse (generations)')}</FormLabel>
                <Select
                  items={windows}
                  value={String(field.value)}
                  onValueChange={(value) => field.onChange(Number(value))}
                  disabled={props.disabled}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {windows.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='adaptive_reasoning.timeout_ms'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Decision timeout (ms)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={30000}
                    {...field}
                    onChange={(event) =>
                      field.onChange(Number(event.target.value))
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='adaptive_reasoning.max_chars'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Decision context limit (characters)')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={60000}
                    {...field}
                    onChange={(event) =>
                      field.onChange(Number(event.target.value))
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <p className='text-muted-foreground text-xs leading-relaxed sm:col-span-2'>
            {t(
              'New user input or tool failures trigger reassessment. Without a stable conversation ID, every request is evaluated. Missing TypeSafe permission or evaluation failure keeps the original effort. Jev calls use the configured channel pricing.'
            )}
          </p>
        </fieldset>
      )}
      <FormField
        control={form.control}
        name='adaptive_reasoning'
        render={() => (
          <FormItem>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}
