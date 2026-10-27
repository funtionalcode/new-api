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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { toast } from 'sonner'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { GlobalSettingsCard } from '../global-settings-card'

function Fixture() {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <SettingsPageProvider actionsContainer={actions}>
      <div ref={setActions} />
      <GlobalSettingsCard
        defaultValues={{
          global: {
            pass_through_request_enabled: false,
            responses_image_tool_model: 'gpt-image-2',
            thinking_model_blacklist: '[]',
            chat_completions_to_responses_policy: '{}',
          },
          general_setting: {
            ping_interval_enabled: false,
            ping_interval_seconds: 60,
          },
        }}
      />
    </SettingsPageProvider>
  )
}

function renderSettings() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Fixture />
    </QueryClientProvider>
  )
  return userEvent.setup()
}

beforeEach(() => {
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

test('editing the saved image model submits only the trimmed model setting', async () => {
  const user = renderSettings()
  const model = screen.getByRole('textbox', {
    name: 'Responses image tool model',
  })
  expect(model).toHaveValue('gpt-image-2')
  await user.clear(model)
  await user.type(model, ' grok-imagine-image-2.0 ')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
  expect(api.put).toHaveBeenCalledWith('/api/option/', {
    key: 'global.responses_image_tool_model',
    value: 'grok-imagine-image-2.0',
  })
})

test('clearing the image model saves an empty value to restore fallback behavior', async () => {
  const user = renderSettings()
  await user.clear(
    screen.getByRole('textbox', { name: 'Responses image tool model' })
  )
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/option/', {
      key: 'global.responses_image_tool_model',
      value: '',
    })
  )
})

test('a failed image model save keeps the draft and allows retry without reporting success', async () => {
  vi.mocked(api.put).mockResolvedValueOnce({
    data: { success: false, message: 'Unable to save image model' },
  })
  const success = vi.spyOn(toast, 'success')
  const error = vi.spyOn(toast, 'error')
  const user = renderSettings()
  const model = screen.getByRole('textbox', {
    name: 'Responses image tool model',
  })
  await user.clear(model)
  await user.type(model, 'grok-imagine-image-2.0')
  const save = screen.getByRole('button', { name: 'Save Changes' })
  await user.click(save)

  await waitFor(() => expect(error).toHaveBeenCalled())
  expect(success).not.toHaveBeenCalled()
  expect(model).toHaveValue('grok-imagine-image-2.0')
  expect(save).toBeEnabled()
  await user.click(save)
  await waitFor(() => expect(api.put).toHaveBeenCalledTimes(2))
  expect(success).toHaveBeenCalled()
})
