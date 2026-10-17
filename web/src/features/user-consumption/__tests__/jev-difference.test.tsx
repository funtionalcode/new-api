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
import { act, render, screen, within } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { expect, it } from 'vitest'

import { JevAuthDifferenceTable } from '../components/jev-auth-difference-table'
import { JevDifferenceSummary } from '../components/jev-difference-summary'
import {
  JevComparisonValue,
  JevDifferenceValue,
} from '../components/jev-difference-value'
import {
  compareJevAuthUsage,
  getJevComparisonMetrics,
  getJevMetricChange,
} from '../lib/jev-comparison'
import { emptyJevMetrics } from '../lib/jev-daily'
import type { JevDailyUsageRow } from '../types'

const baseline: JevDailyUsageRow = {
  ...emptyJevMetrics,
  user_id: 1,
  username: 'alice',
  auth_index: 'auth',
  auth_name: 'a.json',
  provider: 'codex',
  bound: true,
  model_name: 'model',
  jev_status: 'not_used',
  request_count: 10,
  prompt_tokens: 10000,
  completion_tokens: 2000,
  quota: 1000000,
}
const current: JevDailyUsageRow = {
  ...baseline,
  jev_status: 'used',
  request_count: 20,
  completion_tokens: 2000,
  quota: 1000000,
  evaluation_requests: 2,
  evaluation_prompt_tokens: 100,
  evaluation_completion_tokens: 20,
  evaluation_quota: 100000,
}

it('综合消耗包含 Jev 评估开销，每次请求指标按真实次数计算，不依赖档位价格', () => {
  expect(getJevComparisonMetrics(current, 20)).toMatchObject({
    combined_quota: 1100000,
    evaluation_tokens: 120,
    average_input_tokens: 500,
    average_output_tokens: 100,
    average_combined_quota: 55000,
    used_requests: 20,
  })
  expect(getJevMetricChange(100000, 55000)).toEqual({
    difference: -45000,
    percent: -45,
  })
  expect(getJevMetricChange(1000000, 1100000)).toEqual({
    difference: 100000,
    percent: 10,
  })
})

it.each([
  [0, 10, 10, null],
  [10, 0, -10, -100],
  [0, 0, 0, 0],
  [null, 10, null, null],
  [10, null, null, null],
])(
  '基准=%s、分析=%s 时不伪造百分比或无请求均值',
  (before, after, difference, percent) => {
    expect(getJevMetricChange(before, after)).toEqual({ difference, percent })
  }
)

it('无主请求但有独立评估时保留开销，均值明确为不可计算', () => {
  expect(
    getJevComparisonMetrics({ ...emptyJevMetrics, evaluation_quota: 100 }, 0)
  ).toMatchObject({
    combined_quota: 100,
    average_input_tokens: null,
    average_output_tokens: null,
    average_combined_quota: null,
  })
})

it('认证文件对比合并两日并集，同名认证索引按用户隔离，不丢失新增和消失文件', () => {
  const comparisons = compareJevAuthUsage(
    [
      baseline,
      { ...baseline, auth_index: 'removed', auth_name: 'removed.json' },
    ],
    [current, { ...current, user_id: 2, username: 'bob' }]
  )
  expect(comparisons).toHaveLength(3)
  expect(comparisons.find((row) => row.key === '1\0auth')).toMatchObject({
    baseline: { request_count: 10, used_requests: 0 },
    current: { request_count: 20, used_requests: 20 },
  })
  expect(comparisons.find((row) => row.key === '1\0removed')).toMatchObject({
    baseline: { request_count: 10 },
    current: { request_count: 0, average_combined_quota: null },
  })
  expect(comparisons.find((row) => row.key === '2\0auth')).toMatchObject({
    baseline: { request_count: 0 },
    current: { request_count: 20 },
  })
})

it('差值汇总展示日期、金额与增减方向，区分总额增加和单次费用下降', () => {
  render(
    <JevDifferenceSummary
      baseline={{
        start_timestamp: 1792022400,
        end_timestamp: 1792108800,
        log_count: 10,
        rows: [baseline],
      }}
      current={{
        start_timestamp: 1792108800,
        end_timestamp: 1792195200,
        log_count: 20,
        rows: [current],
      }}
    />
  )
  const table = within(
    screen.getByRole('table', { name: 'Day-to-day difference' })
  )
  const costs = within(
    table.getByRole('row', { name: /^Combined cost (?!per request)/ })
  )
  expect(costs.getByText('+10%')).toBeInTheDocument()
  const average = within(
    table.getByRole('row', { name: /^Combined cost per request / })
  )
  expect(average.getByText('-45%')).toBeInTheDocument()
  const tokens = within(
    table.getByRole('row', { name: /^Output tokens per request / })
  )
  expect(tokens.getByText('-50%')).toBeInTheDocument()
  expect(screen.queryByText(/NaN|Infinity/)).not.toBeInTheDocument()
})

it('新增认证文件显示无非零基准，无请求时不展示虚假的零均值或节省', () => {
  render(<JevAuthDifferenceTable baseline={[]} current={[current]} />)
  const table = within(
    screen.getByRole('region', { name: 'Auth file differences' })
  )
  expect(table.getByText('a.json')).toBeInTheDocument()
  expect(table.getAllByText('No nonzero baseline').length).toBeGreaterThan(0)
  expect(table.getAllByText('No requests').length).toBeGreaterThan(0)
  expect(screen.queryByText(/NaN|Infinity/)).not.toBeInTheDocument()
})

it.each([
  ['zhCN', '1,234.5'],
  ['zhTW', '1,234.5'],
  ['en', '1,234.5'],
  ['fr', '1\u202f234,5'],
  ['ja', '1,234.5'],
  ['ru', '1\u00a0234,5'],
  ['vi', '1.234,5'],
  ['invalid', '1,234.5'],
])(
  '差值在界面语言 %s 下正确格式化，语言切换后同步更新',
  async (language, expected) => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: language,
      fallbackLng: 'en',
      resources: {
        [language]: { translation: { 'No requests': 'No requests' } },
        en: { translation: {} },
      },
    })
    render(
      <I18nextProvider i18n={i18n}>
        <JevComparisonValue value={1234.5} />
        <JevDifferenceValue baseline={100} current={150} />
      </I18nextProvider>
    )
    expect(
      screen.getByText(expected, { normalizer: (value) => value })
    ).toBeInTheDocument()
    expect(screen.getByText('+50%')).toBeInTheDocument()
    await act(async () => {
      await i18n.changeLanguage('en')
    })
    expect(screen.getByText('1,234.5')).toBeInTheDocument()
  }
)
