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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { getJevDailyUsage } from '../api'
import { JevDailyComparison } from '../components/jev-daily-comparison'
import { UserConsumption } from '../index'
import {
  emptyJevMetrics,
  getJevDayRange,
  groupJevUsageByAuth,
  sumJevUsage,
} from '../lib/jev-daily'
import type { JevDailyUsageRow } from '../types'

// jsdom 不提供 Canvas；只替换第三方绘制边界，保留页面、筛选和请求逻辑。
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

const rows: JevDailyUsageRow[] = [
  {
    ...emptyJevMetrics,
    user_id: 11,
    username: 'alice',
    auth_index: 'auth-a',
    auth_name: 'a.json',
    provider: 'codex',
    bound: true,
    model_name: 'model-a',
    jev_status: 'used',
    request_count: 2,
    completion_tokens: 20,
    quota: 200,
    cached_decisions: 1,
    evaluation_requests: 1,
    evaluation_quota: 5,
  },
  {
    ...emptyJevMetrics,
    user_id: 11,
    username: 'alice',
    auth_index: 'auth-a',
    auth_name: 'a.json',
    provider: 'codex',
    bound: true,
    model_name: 'model-b',
    jev_status: 'not_used',
    request_count: 1,
    completion_tokens: 100,
    quota: 1000,
  },
  {
    ...emptyJevMetrics,
    user_id: 12,
    username: 'bob',
    auth_index: 'auth-a',
    auth_name: '',
    provider: '',
    bound: false,
    model_name: 'model-a',
    jev_status: 'not_applied',
    request_count: 1,
    quota: 10,
  },
  {
    ...emptyJevMetrics,
    user_id: 11,
    username: 'alice',
    auth_index: 'idle',
    auth_name: 'idle.json',
    provider: 'codex',
    bound: true,
    model_name: '',
    jev_status: 'no_activity',
  },
  {
    ...emptyJevMetrics,
    user_id: 11,
    username: 'alice',
    auth_index: 'auth-a',
    auth_name: 'a.json',
    provider: 'codex',
    bound: true,
    model_name: '',
    jev_status: 'evaluation_only',
    evaluation_requests: 1,
    evaluation_quota: 3,
  },
]

function renderReport() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <JevDailyComparison />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  vi.spyOn(api, 'get').mockImplementation(async (_url, config) => ({
    data: {
      success: true,
      data: { ...config?.params, log_count: 6, rows },
    },
  }))
})

it('认证文件汇总保留同名令牌的用户隔离、零消耗文件，并分别计算主请求和评估成本', () => {
  const files = groupJevUsageByAuth(rows)
  expect(files).toHaveLength(3)
  expect(files[0]).toMatchObject({
    user_id: 11,
    auth_index: 'auth-a',
    used_requests: 2,
    not_used_requests: 1,
    request_count: 3,
    quota: 1200,
    evaluation_requests: 2,
    evaluation_quota: 8,
  })
  expect(files.find((file) => file.user_id === 12)).toMatchObject({
    not_applied_requests: 1,
    quota: 10,
    bound: false,
  })
  expect(files.find((file) => file.auth_index === 'idle')).toMatchObject({
    request_count: 0,
    quota: 0,
  })
  expect(sumJevUsage(rows)).toMatchObject({
    request_count: 4,
    quota: 1210,
    evaluation_quota: 8,
    completion_tokens: 120,
  })
  expect(sumJevUsage([])).toEqual(emptyJevMetrics)
})

it('自然日采用本地午夜到次日午夜的半开区间', () => {
  const date = new Date(2026, 9, 16, 14, 35)
  expect(getJevDayRange(date)).toEqual({
    start_timestamp: new Date(2026, 9, 16, 0, 0, 0).getTime() / 1000,
    end_timestamp: new Date(2026, 9, 17, 0, 0, 0).getTime() / 1000,
  })
  expect(date.getHours()).toBe(14)
})

it('按认证文件和模型筛选时汇总同步变化，不把未匹配评估强行归给模型', async () => {
  renderReport()
  expect((await screen.findAllByText('a.json')).length).toBeGreaterThan(0)
  expect(screen.getAllByText('idle.json').length).toBeGreaterThan(0)
  expect(
    screen.getAllByText('No matching auth binding').length
  ).toBeGreaterThan(0)
  const authTable = screen.getAllByRole('table')[3]
  expect(within(authTable).getAllByRole('row')).toHaveLength(4)
  await userEvent.type(screen.getByLabelText('User or auth file'), 'a.json')
  expect(screen.queryByText('idle.json')).not.toBeInTheDocument()
  expect(screen.queryByText('No matching auth binding')).not.toBeInTheDocument()
  await userEvent.selectOptions(screen.getByLabelText('Model'), 'model-a')
  expect(
    screen.getByText(
      'Model filtering excludes standalone or unlinked Jev costs because their main model cannot be confirmed.'
    )
  ).toBeInTheDocument()
  expect(
    within(screen.getAllByRole('table')[3]).getAllByRole('row')
  ).toHaveLength(2)
  expect(api.get).toHaveBeenCalledTimes(2)
  const summary = within(
    screen.getByRole('table', { name: 'Day-to-day difference' })
  )
  expect(
    summary.getByRole('row', { name: /^Main requests / })
  ).toHaveTextContent('2')
})

it('默认对比最近两个完整自然日，关闭后仅展示单日报表', async () => {
  renderReport()
  await screen.findByRole('table', { name: 'Day-to-day difference' })
  expect(
    screen.getByRole('switch', { name: 'Compare another day' })
  ).toBeChecked()
  expect(screen.getAllByRole('table')).toHaveLength(6)
  const starts = vi
    .mocked(api.get)
    .mock.calls.map(([, config]) => config?.params?.start_timestamp)
  expect(new Set(starts).size).toBe(2)
  const today = getJevDayRange(new Date()).start_timestamp
  expect(starts.every((start) => start < today)).toBe(true)
  await userEvent.click(
    screen.getByRole('switch', { name: 'Compare another day' })
  )
  await waitFor(() => expect(screen.getAllByRole('table')).toHaveLength(2))
  expect(
    screen.queryByRole('table', { name: 'Day-to-day difference' })
  ).not.toBeInTheDocument()
})

it('两日报表经过同一模型筛选后，页面差值随之更新而不是继续展示全量差值', async () => {
  const analysisStart = getJevDayRange(
    new Date(new Date().setDate(new Date().getDate() - 1))
  ).start_timestamp
  vi.mocked(api.get).mockImplementation(async (_url, config) => ({
    data: {
      success: true,
      data: {
        ...config?.params,
        log_count: 2,
        rows: rows.slice(0, 2).map((row) => ({
          ...row,
          completion_tokens:
            config?.params?.start_timestamp === analysisStart
              ? row.completion_tokens / 2
              : row.completion_tokens,
        })),
      },
    },
  }))
  renderReport()
  await screen.findByRole('table', { name: 'Day-to-day difference' })
  const getOutputRow = () =>
    within(
      screen.getByRole('table', { name: 'Day-to-day difference' })
    ).getByRole('row', { name: /^Main output tokens / })
  expect(within(getOutputRow()).getByText('-60')).toBeInTheDocument()
  await userEvent.selectOptions(screen.getByLabelText('Model'), 'model-a')
  expect(within(getOutputRow()).getByText('-10')).toBeInTheDocument()
  expect(within(getOutputRow()).getByText('-50%')).toBeInTheDocument()
  expect(api.get).toHaveBeenCalledTimes(2)
})

it('同一天只读取一次并提示不能计算跨日差值', async () => {
  renderReport()
  await screen.findByRole('table', { name: 'Day-to-day difference' })
  await userEvent.click(
    within(screen.getByRole('region', { name: 'Baseline date' })).getByRole(
      'button'
    )
  )
  const target = new Date()
  target.setDate(target.getDate() - 1)
  const month = new Intl.DateTimeFormat('en', { month: 'long' }).format(target)
  const dateName = new RegExp(
    `${month} ${target.getDate()}(?:st|nd|rd|th)?, ${target.getFullYear()}`
  )
  await userEvent.click(screen.getByRole('button', { name: dateName }))
  expect(
    await screen.findByText(
      'Choose two different dates to calculate a difference.'
    )
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('table', { name: 'Day-to-day difference' })
  ).not.toBeInTheDocument()
  expect(api.get).toHaveBeenCalledTimes(3)
})

it('选择今天时提示数据未满一天，不把当前累计当作完整日消耗', async () => {
  renderReport()
  await screen.findByRole('table', { name: 'Day-to-day difference' })
  await userEvent.click(
    within(screen.getByRole('region', { name: 'Analysis date' })).getByRole(
      'button'
    )
  )
  const today = new Date()
  const month = new Intl.DateTimeFormat('en', { month: 'long' }).format(today)
  const dateName = new RegExp(
    `${month} ${today.getDate()}(?:st|nd|rd|th)?, ${today.getFullYear()}`
  )
  await userEvent.click(screen.getByRole('button', { name: dateName }))
  expect(
    await screen.findByText(
      'One selected day is incomplete. Its totals are provisional.'
    )
  ).toBeInTheDocument()
  expect(
    screen.getByRole('table', { name: 'Day-to-day difference' })
  ).toBeInTheDocument()
})

it('API 业务失败展示可重试错误，不把失败当作零消耗', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: { success: false, message: 'daily limit exceeded' },
  })
  renderReport()
  expect(await screen.findByText('daily limit exceeded')).toBeInTheDocument()
  expect(screen.queryByRole('table')).not.toBeInTheDocument()
  vi.mocked(api.get).mockImplementation(async (_url, config) => ({
    data: { success: true, data: { ...config?.params, log_count: 6, rows } },
  }))
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  expect((await screen.findAllByText('a.json')).length).toBeGreaterThan(0)
})

it('空日报表仍明确展示零值且不生成无穷平均数', async () => {
  vi.mocked(api.get).mockImplementation(async (_url, config) => ({
    data: {
      success: true,
      data: {
        ...config?.params,
        log_count: 0,
        rows: [],
      },
    },
  }))
  renderReport()
  expect((await screen.findAllByText('No Data')).length).toBeGreaterThan(0)
  expect(screen.getAllByRole('table')).toHaveLength(6)
  expect(screen.queryByText(/NaN|Infinity/)).not.toBeInTheDocument()
})

it('获取日报时转发取消信号并拒绝缺失的数据', async () => {
  const signal = new AbortController().signal
  await getJevDailyUsage({ start_timestamp: 100, end_timestamp: 86500 }, signal)
  expect(api.get).toHaveBeenCalledWith('/api/cliproxy/jev-daily-usage', {
    params: { start_timestamp: 100, end_timestamp: 86500 },
    signal,
  })
  vi.mocked(api.get).mockResolvedValueOnce({ data: { success: true } })
  await expect(
    getJevDailyUsage({ start_timestamp: 100, end_timestamp: 86500 })
  ).rejects.toThrow('Failed to load Jev daily usage')
})

it('用户消耗页切换到日报后才查询日报，保留可滚动内容区域', async () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <UserConsumption />
    </QueryClientProvider>
  )
  await waitFor(() => expect(api.get).toHaveBeenCalled())
  expect(
    vi
      .mocked(api.get)
      .mock.calls.some(([url]) => String(url).includes('jev-daily-usage'))
  ).toBe(false)
  await userEvent.click(
    screen.getByRole('tab', { name: 'Jev daily comparison' })
  )
  expect((await screen.findAllByText('a.json')).length).toBeGreaterThan(0)
  const panel = screen.getByRole('tabpanel', { name: 'Jev daily comparison' })
  expect(panel).toHaveClass('min-h-0', 'overflow-hidden')
  expect(within(panel).getByRole('main')).toHaveClass('min-h-0', 'flex-1')
})
