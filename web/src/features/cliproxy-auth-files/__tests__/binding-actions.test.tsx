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
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { CliproxyAuthFiles } from '../index'

let files = [
  { authIndex: 'bound', name: 'bound.json', enabled: false, bound: true },
  { authIndex: 'free', name: 'free.json', enabled: true, bound: false },
]

beforeEach(() => {
  files = [
    { authIndex: 'bound', name: 'bound.json', enabled: false, bound: true },
    { authIndex: 'free', name: 'free.json', enabled: true, bound: false },
  ]
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 10 })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: String(url).endsWith('/remote')
        ? files
        : {
            items: String(url).includes('/user/search')
              ? [{ id: 2, username: 'qa-user' }]
              : [],
            total: 0,
          },
    },
  }))
})

afterEach(() => useAuthStore.getState().auth.reset())

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <CliproxyAuthFiles />
    </QueryClientProvider>
  )
  return client
}

it('已绑定且已禁用的文件显示不可点击的已绑定按钮，筛选列表不会解锁', async () => {
  renderPage()
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  const bound = within(
    await screen.findByRole('row', { name: /^bound\.json/ })
  ).getByRole('button', { name: 'Bound' })
  expect(bound).toBeDisabled()
  await userEvent.click(bound)
  bound.focus()
  await userEvent.keyboard('{Enter} ')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  await userEvent.type(
    screen.getByPlaceholderText('Filter by username'),
    'another-user'
  )
  expect(bound).toBeDisabled()
  expect(
    within(screen.getByRole('row', { name: /^free\.json/ })).getByRole(
      'button',
      {
        name: 'Bind User',
      }
    )
  ).toBeEnabled()
})

it('新增绑定成功后立即禁用按钮，远端刷新结束后仍保持已绑定', async () => {
  const client = renderPage()
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  await screen.findByText('free.json')
  let bindingSaved = false
  let resolveRecheck:
    | ((data: { data: { success: boolean; data: typeof files } }) => void)
    | undefined
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).endsWith('/remote') && bindingSaved) {
      return await new Promise((resolve) => {
        resolveRecheck = resolve
      })
    }
    return {
      data: {
        success: true,
        data: { items: [{ id: 2, username: 'qa-user' }], total: 1 },
      },
    }
  })
  const post = vi.spyOn(api, 'post').mockImplementation(async () => {
    bindingSaved = true
    files = files.map((file) =>
      file.authIndex === 'free' ? { ...file, bound: true } : file
    )
    return { data: { success: true, data: { id: 3, auth_index: 'free' } } }
  })
  await userEvent.click(screen.getByRole('button', { name: 'Bind User' }))
  await userEvent.click(await screen.findByRole('button', { name: 'qa-user' }))
  await userEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', { name: 'Save' })
  )
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  const button = within(
    screen.getByRole('row', { name: /^free\.json/ })
  ).getByRole('button', { name: 'Bound' })
  expect(button).toBeDisabled()
  await act(async () => {
    resolveRecheck?.({ data: { success: true, data: files } })
  })
  await waitFor(() => expect(client.isFetching()).toBe(0))
  expect(button).toBeDisabled()
  expect(post).toHaveBeenCalledTimes(1)
})

it('刷新确认解绑后恢复绑定按钮，刷新期间不可点击旧的未绑定按钮', async () => {
  const client = renderPage()
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  await screen.findByText('free.json')
  let resolveRemote:
    | ((data: { data: { success: boolean; data: typeof files } }) => void)
    | undefined
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).endsWith('/remote')) {
      return await new Promise((resolve) => {
        resolveRemote = resolve
      })
    }
    return { data: { success: true, data: { items: [], total: 0 } } }
  })
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  expect(screen.getByRole('button', { name: 'Bind User' })).toBeDisabled()
  files = files.map((file) => ({ ...file, bound: false }))
  await act(async () => {
    resolveRemote?.({ data: { success: true, data: files } })
  })
  await waitFor(() => expect(client.isFetching()).toBe(0))
  const button = within(
    screen.getByRole('row', { name: /^bound\.json/ })
  ).getByRole('button', { name: 'Bind User' })
  expect(button).toBeEnabled()
  await userEvent.click(button)
  expect(await screen.findByRole('dialog')).toBeInTheDocument()
})

it('刷新失败时不可使用旧的未绑定状态继续绑定', async () => {
  const client = renderPage()
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  await screen.findByText('free.json')
  vi.spyOn(api, 'get').mockRejectedValue(new Error('test-only request failure'))
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  await waitFor(() => expect(client.isFetching()).toBe(0))
  const button = within(
    screen.getByRole('row', { name: /^free\.json/ })
  ).getByRole('button', { name: 'Bind User' })
  expect(button).toBeDisabled()
  await userEvent.click(button)
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('保存绑定失败时保留未绑定状态，取消后仍可重新绑定', async () => {
  renderPage()
  await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  await screen.findByText('free.json')
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: false, message: 'test-only save failure' },
  })
  await userEvent.click(screen.getByRole('button', { name: 'Bind User' }))
  await userEvent.click(await screen.findByRole('button', { name: 'qa-user' }))
  const dialog = screen.getByRole('dialog')
  await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(within(dialog).getByRole('button', { name: 'Save' })).toBeEnabled()
  )
  expect(post).toHaveBeenCalledTimes(1)
  await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  const button = within(
    screen.getByRole('row', { name: /^free\.json/ })
  ).getByRole('button', { name: 'Bind User' })
  expect(button).toBeEnabled()
})
