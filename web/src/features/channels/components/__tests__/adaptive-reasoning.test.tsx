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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useForm } from 'react-hook-form'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { Form } from '@/components/ui/form'

import { searchChannels } from '../../api'
import {
  ADAPTIVE_REASONING_DEFAULTS,
  adaptiveReasoningSchema,
  type AdaptiveReasoningConfig,
} from '../../lib/adaptive-reasoning'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  buildSettingJSON,
  transformChannelToFormDefaults,
  type ChannelFormValues,
} from '../../lib/channel-form'
import { isAdvancedSettingsField } from '../../lib/channel-form-errors'
import { channelSchema } from '../../types'
import { AdaptiveReasoningSettings } from '../adaptive-reasoning-settings'

vi.mock('../../api', () => ({ searchChannels: vi.fn() }))

function SettingsForm(props: {
  config?: AdaptiveReasoningConfig
  type?: number
  disabled?: boolean
  onSave?: (value: string) => void
}) {
  const form = useForm<ChannelFormValues>({
    defaultValues: {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: props.type ?? 62,
      adaptive_reasoning: props.config ?? { ...ADAPTIVE_REASONING_DEFAULTS },
    },
  })
  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit((values) =>
          props.onSave?.(buildSettingJSON(values))
        )}
      >
        <AdaptiveReasoningSettings
          channelType={props.type ?? 62}
          disabled={props.disabled}
        />
        <button type='submit'>Save</button>
      </form>
    </Form>
  )
}

function renderSettings(props: Parameters<typeof SettingsForm>[0] = {}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <SettingsForm {...props} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.mocked(searchChannels).mockReset()
  vi.mocked(searchChannels).mockResolvedValue({
    success: true,
    message: '',
    data: {
      items: [
        channelSchema.parse({
          key: '',
          created_time: 0,
          test_time: 0,
          response_time: 0,
          balance_updated_time: 0,
          id: 21,
          type: 66,
          name: 'Jev evaluator',
          status: 1,
          models: 'jev-latest',
        }),
      ],
      total: 1,
    },
  })
})

describe('adaptive reasoning channel configuration', () => {
  test('starts disabled and loads evaluator channels only after enabling', async () => {
    const user = userEvent.setup()
    const save = vi.fn()
    renderSettings({ onSave: save })
    expect(
      screen.getByRole('switch', { name: 'Enable adaptive reasoning' })
    ).not.toBeChecked()
    expect(searchChannels).not.toHaveBeenCalled()
    expect(
      screen.queryByRole('textbox', { name: 'Evaluation Model' })
    ).not.toBeInTheDocument()
    await user.click(
      screen.getByRole('switch', { name: 'Enable adaptive reasoning' })
    )
    const select = await screen.findByRole('combobox', {
      name: 'TypeSafe channel',
    })
    await user.click(select)
    await user.click(
      await screen.findByRole('option', { name: 'Jev evaluator (#21)' })
    )
    expect(
      screen.getByRole('textbox', { name: 'Evaluation Model' })
    ).toHaveValue('jev-latest')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(save).toHaveBeenCalledOnce()
    expect(JSON.parse(save.mock.calls[0][0]).adaptive_reasoning).toEqual({
      ...ADAPTIVE_REASONING_DEFAULTS,
      enabled: true,
      channel_id: 21,
    })
    expect(searchChannels).toHaveBeenCalledWith(
      expect.objectContaining({ type: 66 })
    )
  })

  test('disabling retains the configured evaluator for the next edit', async () => {
    const user = userEvent.setup()
    const save = vi.fn()
    renderSettings({
      config: { ...ADAPTIVE_REASONING_DEFAULTS, enabled: true, channel_id: 21 },
      onSave: save,
    })
    await user.click(
      screen.getByRole('switch', { name: 'Enable adaptive reasoning' })
    )
    expect(
      screen.queryByRole('textbox', { name: 'Evaluation Model' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(JSON.parse(save.mock.calls[0][0]).adaptive_reasoning).toMatchObject({
      enabled: false,
      channel_id: 21,
    })
  })

  test('sensitive fields remain disabled when editing is locked', () => {
    renderSettings({
      disabled: true,
      config: { ...ADAPTIVE_REASONING_DEFAULTS, enabled: true, channel_id: 21 },
    })
    expect(
      screen.getByRole('switch', { name: 'Enable adaptive reasoning' })
    ).toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByRole('textbox', { name: 'Evaluation Model' })
    ).toBeDisabled()
    expect(
      screen.getByRole('spinbutton', { name: 'Decision timeout (ms)' })
    ).toBeDisabled()
    expect(searchChannels).not.toHaveBeenCalled()
  })

  test('unsupported channel types hide adaptive controls and omit their settings', () => {
    renderSettings({ type: 14 })
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    const value = buildSettingJSON({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: 14,
      adaptive_reasoning: {
        ...ADAPTIVE_REASONING_DEFAULTS,
        enabled: true,
        channel_id: 21,
      },
    })
    expect(JSON.parse(value).adaptive_reasoning).toBeUndefined()
  })

  test('saved configuration survives loading and serialization', () => {
    const config = {
      ...ADAPTIVE_REASONING_DEFAULTS,
      enabled: true,
      channel_id: 21,
      max_reuse_generations: 5,
      timeout_ms: 2500,
      max_chars: 6000,
    }
    const channel = channelSchema.parse({
      key: '',
      status: 1,
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      id: 18,
      name: 'Astra',
      type: 62,
      setting: JSON.stringify({ adaptive_reasoning: config }),
    })
    const form = transformChannelToFormDefaults(channel)
    expect(form.adaptive_reasoning).toEqual(config)
    expect(
      JSON.parse(buildSettingJSON(form as ChannelFormValues)).adaptive_reasoning
    ).toEqual(config)
  })

  test('enabled configuration requires a channel and allowed effort and routes nested errors to advanced settings', () => {
    expect(
      adaptiveReasoningSchema.safeParse({
        ...ADAPTIVE_REASONING_DEFAULTS,
        enabled: true,
        channel_id: 0,
      }).success
    ).toBe(false)
    expect(
      adaptiveReasoningSchema.safeParse({
        ...ADAPTIVE_REASONING_DEFAULTS,
        enabled: true,
        channel_id: 21,
        efforts: [],
      }).success
    ).toBe(false)
    expect(
      adaptiveReasoningSchema.safeParse({
        ...ADAPTIVE_REASONING_DEFAULTS,
        max_reuse_generations: 3,
      }).success
    ).toBe(false)
    expect(isAdvancedSettingsField('adaptive_reasoning.channel_id')).toBe(true)
  })
})
