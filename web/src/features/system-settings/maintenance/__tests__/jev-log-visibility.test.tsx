import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { LogSettingsSection } from '../log-settings-section'

function Fixture(props: { showJevLogs?: boolean }) {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setContainer} />
      <SettingsPageProvider actionsContainer={container}>
        <LogSettingsSection
          defaultEnabled
          defaultShowJevLogs={props.showJevLogs}
        />
      </SettingsPageProvider>
    </>
  )
}

function renderSettings(showJevLogs?: boolean) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  client.setQueryData(['logs', 'common'], { items: [] })
  render(
    <QueryClientProvider client={client}>
      <Fixture showJevLogs={showJevLogs} />
    </QueryClientProvider>
  )
  return client
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/performance/logs' ? { enabled: false } : null,
    },
  }))
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

afterEach(() => vi.restoreAllMocks())

test('missing setting defaults to visible and saving hidden refreshes cached logs', async () => {
  const client = renderSettings()
  const toggle = screen.getByRole('switch', { name: 'Show Jev model logs' })
  expect(toggle).toBeChecked()
  fireEvent.click(toggle)
  fireEvent.click(screen.getByRole('button', { name: 'Save log settings' }))
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/option/', {
      key: 'general_setting.show_jev_logs',
      value: false,
    })
  )
  await waitFor(() =>
    expect(client.getQueryState(['logs', 'common'])?.isInvalidated).toBe(true)
  )
  expect(
    screen.getByRole('switch', { name: 'Record quota usage' })
  ).toBeChecked()
  expect(api.put).toHaveBeenCalledTimes(1)
})

test('saved hidden setting can be switched back on without changing log recording', async () => {
  renderSettings(false)
  const toggle = screen.getByRole('switch', { name: 'Show Jev model logs' })
  expect(toggle).not.toBeChecked()
  fireEvent.click(toggle)
  fireEvent.click(screen.getByRole('button', { name: 'Save log settings' }))
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith('/api/option/', {
      key: 'general_setting.show_jev_logs',
      value: true,
    })
  )
  expect(api.put).toHaveBeenCalledTimes(1)
})

test('saving an unchanged visibility setting sends no update', async () => {
  renderSettings(false)
  fireEvent.click(screen.getByRole('button', { name: 'Save log settings' }))
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Save log settings' })
    ).toBeEnabled()
  )
  expect(api.put).not.toHaveBeenCalled()
})

test('failed save preserves the selected visibility and can be retried', async () => {
  vi.mocked(api.put).mockRejectedValueOnce(new Error('Save failed'))
  const client = renderSettings()
  const toggle = screen.getByRole('switch', { name: 'Show Jev model logs' })
  fireEvent.click(toggle)
  fireEvent.click(screen.getByRole('button', { name: 'Save log settings' }))
  await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Save log settings' })
    ).toBeEnabled()
  )
  expect(toggle).not.toBeChecked()
  expect(client.getQueryState(['logs', 'common'])?.isInvalidated).toBe(false)
  fireEvent.click(screen.getByRole('button', { name: 'Save log settings' }))
  await waitFor(() =>
    expect(client.getQueryState(['logs', 'common'])?.isInvalidated).toBe(true)
  )
  expect(api.put).toHaveBeenLastCalledWith('/api/option/', {
    key: 'general_setting.show_jev_logs',
    value: false,
  })
})
