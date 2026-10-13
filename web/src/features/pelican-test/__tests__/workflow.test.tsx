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
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { PelicanTest } from '../index'
import {
  historyStorageKey,
  PELICAN_TEST_PROMPT,
  readPelicanTestHistory,
  writePelicanTestHistory,
  type PelicanTestRecord,
} from '../lib'

const html =
  '<!doctype html><html><body><svg viewBox="0 0 200 100"><circle r="10"/><animate attributeName="cx" values="0;100" dur="2s" repeatCount="indefinite"/></svg></body></html>'

function savedRecord(id: string, model = 'model-a'): PelicanTestRecord {
  return {
    id,
    model,
    group: 'default',
    createdAt: 1720000000000,
    status: 'success',
    content: html,
    assessments: [],
  }
}

function renderWorkspace() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PelicanTest />
    </QueryClientProvider>
  )
}

function completion(content: string, finishReason = 'stop') {
  return {
    data: { choices: [{ message: { content }, finish_reason: finishReason }] },
    headers: {},
  }
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser({ id: 17, username: 'alice', role: 1 })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: String(url).endsWith('/groups')
        ? {
            default: { desc: 'Default', ratio: 1 },
            vip: { desc: 'VIP', ratio: 1 },
          }
        : ['model-a', 'model-b', 'grok-imagine-video-1.5'],
    },
  }))
})

afterEach(async () => {
  useAuthStore.getState().auth.reset()
  await i18next.changeLanguage('en')
})

it('选择生成模型后发送原始固定提示词，成功结果持久化并可显式加载隔离预览', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue(completion(html))
  renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  const generation = screen.getByRole('region', { name: 'Generation model' })
  await userEvent.click(within(generation).getAllByRole('combobox')[0])
  await userEvent.click(screen.getByRole('option', { name: 'model-b' }))
  await userEvent.click(submit)
  await screen.findByText('SVG detected')
  expect(post).toHaveBeenCalledWith(
    '/pg/chat/completions',
    {
      model: 'model-b',
      group: 'default',
      messages: [{ role: 'user', content: PELICAN_TEST_PROMPT }],
      stream: false,
    },
    expect.objectContaining({ signal: expect.any(AbortSignal) })
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    model: 'model-b',
    status: 'success',
    content: html,
  })
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Load preview' }))
  const frame = screen.getByTitle('Pelican HTML preview')
  expect(frame).toHaveAttribute('sandbox', 'allow-scripts')
  expect(frame).toHaveAttribute('referrerpolicy', 'no-referrer')
  expect(frame.getAttribute('srcdoc')).toContain("connect-src 'none'")
  await userEvent.click(screen.getByRole('button', { name: 'Stop preview' }))
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
})

it('上游失败时保存失败历史及具体原因，并且不自动重试', async () => {
  const post = vi.spyOn(api, 'post').mockRejectedValue({
    response: {
      data: { error: { message: 'unknown provider for model model-a' } },
    },
  })
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'unknown provider for model model-a'
  )
  expect(post).toHaveBeenCalledTimes(1)
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'error',
    errorMessage: 'unknown provider for model model-a',
  })
  expect(
    screen.getByRole('button', { name: 'Assess current result' })
  ).toBeDisabled()
})

it('停止在途生成后保留取消历史，晚到的成功响应不会覆盖状态', async () => {
  let resolveResponse: (response: ReturnType<typeof completion>) => void = () =>
    undefined
  vi.spyOn(api, 'post').mockImplementation(
    () =>
      new Promise((resolve) => {
        resolveResponse = resolve
      })
  )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  await userEvent.click(await screen.findByRole('button', { name: 'Stop' }))
  await act(async () => resolveResponse(completion(html)))
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0].status).toBe(
    'cancelled'
  )
  expect(screen.queryByText('SVG detected')).not.toBeInTheDocument()
})

it('选择评审模型后对当前结果生成评审，保存评审模型和历史并保留原始结果', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('old')])
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue(
      completion('鹈鹕 8/10；代码包含循环 SVG 动画；实际流畅性未验证')
    )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Assess current result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    within(screen.getByRole('region', { name: 'Assessment model' })).getByRole(
      'combobox'
    )
  )
  await userEvent.click(screen.getByRole('option', { name: 'model-b' }))
  await userEvent.click(
    screen.getByRole('button', { name: 'Assess current result' })
  )
  await screen.findByRole('region', { name: 'Assessment history' })
  expect(post).toHaveBeenCalledWith(
    '/pg/chat/completions',
    expect.objectContaining({
      model: 'model-b',
      stream: false,
      messages: [
        expect.objectContaining({ role: 'system' }),
        {
          role: 'user',
          content: JSON.stringify({
            task: PELICAN_TEST_PROMPT,
            generation_model: 'model-a',
            html,
          }),
        },
      ],
    }),
    expect.any(Object)
  )
  const saved = readPelicanTestHistory(historyStorageKey(17))[0]
  expect(saved.content).toBe(html)
  expect(saved.assessments[0]).toMatchObject({
    model: 'model-b',
    status: 'success',
  })
})

it('勾选多条历史后只评审成功结果，并允许标记质量和对比选中结果', async () => {
  const failed = {
    ...savedRecord('failed'),
    status: 'error' as const,
    content: '',
  }
  writePelicanTestHistory(historyStorageKey(17), [
    savedRecord('a'),
    savedRecord('b', 'model-b'),
    failed,
  ])
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue(completion('代码评审结果'))
  renderWorkspace()
  await screen.findAllByRole('checkbox')
  for (const checkbox of screen.getAllByRole('checkbox')) {
    await userEvent.click(checkbox)
  }
  expect(
    await screen.findByRole('region', { name: 'Compare selected results' })
  ).toBeVisible()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Assess selected history (2)' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Assess selected history (2)' })
  )
  await waitFor(() =>
    expect(
      readPelicanTestHistory(historyStorageKey(17)).filter(
        (record) => record.assessments.length
      ).length
    ).toBe(2)
  )
  expect(post).toHaveBeenCalledTimes(2)
  await userEvent.click(
    screen.getAllByRole('button', { name: 'Good result' })[0]
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0].rating).toBe('good')
})

it('界面语言切换为简体与繁体中文时，日期显示和已保存历史保持可用', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('existing')])
  renderWorkspace()
  await screen.findByRole('region', { name: 'Result for model-a' })
  await act(async () => i18next.changeLanguage('zhCN'))
  expect(
    screen.getByRole('region', { name: 'Result for model-a' })
  ).toBeVisible()
  await act(async () => i18next.changeLanguage('zhTW'))
  expect(
    screen.getByRole('region', { name: 'Result for model-a' })
  ).toBeVisible()
})

it('存储写入失败时保留当前结果并提示未保存，而不是报告请求失败', async () => {
  vi.spyOn(api, 'post').mockResolvedValue(completion(html))
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
    throw new Error('storage quota')
  })
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  await screen.findByText('SVG detected')
  expect(screen.getByRole('alert')).toHaveTextContent(
    'History could not be saved'
  )
})

it('切换用户时不会显示或写入另一位用户的测试历史', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('alice')])
  writePelicanTestHistory(historyStorageKey(18), [
    savedRecord('bob', 'model-b'),
  ])
  renderWorkspace()
  await screen.findByRole('region', { name: 'Result for model-a' })
  act(() =>
    useAuthStore.getState().auth.setUser({ id: 18, username: 'bob', role: 1 })
  )
  await screen.findByRole('region', { name: 'Result for model-b' })
  expect(
    screen.queryByRole('region', { name: 'Result for model-a' })
  ).not.toBeInTheDocument()
  expect(readPelicanTestHistory(historyStorageKey(17))[0].id).toBe('alice')
})

it('模型返回截断 HTML 时保留原始内容但标记失败，不提供成功预览或评审', async () => {
  vi.spyOn(api, 'post').mockResolvedValue(completion('<html><svg>', 'length'))
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'The model response was truncated'
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'error',
    content: '<html><svg>',
  })
  expect(
    screen.queryByRole('button', { name: 'Load preview' })
  ).not.toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Assess current result' })
  ).toBeDisabled()
})

it('模型列表获取失败时禁止提交，并允许重试获取列表', async () => {
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).endsWith('/models')) throw new Error('unavailable models')
    return {
      data: { success: true, data: { default: { desc: 'Default', ratio: 1 } } },
    }
  })
  const post = vi.spyOn(api, 'post')
  renderWorkspace()
  await screen.findByText('Failed to load models')
  expect(
    screen.getByRole('button', { name: 'Generate and save result' })
  ).toBeDisabled()
  get.mockImplementation(async (url) => ({
    data: {
      success: true,
      data: String(url).endsWith('/models')
        ? ['model-a']
        : { default: { desc: 'Default', ratio: 1 } },
    },
  }))
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  expect(post).not.toHaveBeenCalled()
})

it('已有历史损坏时提示错误且禁止新任务覆盖原数据', async () => {
  localStorage.setItem(historyStorageKey(17), '{bad history')
  renderWorkspace()
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Test history could not be loaded'
  )
  expect(
    screen.getByRole('button', { name: 'Generate and save result' })
  ).toBeDisabled()
  expect(localStorage.getItem(historyStorageKey(17))).toBe('{bad history')
})

it('查看不同历史结果时不会沿用之前的预览执行状态', async () => {
  writePelicanTestHistory(historyStorageKey(17), [
    savedRecord('a'),
    savedRecord('b', 'model-b'),
  ])
  renderWorkspace()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Load preview' })
  )
  expect(screen.getByTitle('Pelican HTML preview')).toBeInTheDocument()
  await userEvent.click(
    screen.getByRole('button', { name: /model-b.*Completed/ })
  )
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Load preview' })).toBeEnabled()
})
