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
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { expect, test } from 'vitest'

import { BindingUsageCell } from '../../index'
import type { CliproxyAuthFileBinding } from '../../types'
import { CodexCreditsDetails } from '../codex-credits-details'

const binding: CliproxyAuthFileBinding = {
  id: 1,
  user_id: 1,
  username: 'root',
  remark: '',
  auth_index: 'codex-auth',
  auth_name: 'codex-account.json',
  auth_file: 'codex-account.json',
  provider: 'codex',
  description: '',
  account_id: '',
  enabled: true,
  last_refreshed_at: 1794820000,
  last_usage_tokens: 0,
  last_usage_quota: 0,
  last_plan_type: 'pro',
  last_five_hour_percent: 40,
  last_five_hour_reset_at: 1794840000,
  last_weekly_percent: 60,
  last_weekly_reset_at: 1795000000,
  last_codex_five_hour_percent: 0,
  last_codex_five_hour_reset_at: 0,
  last_codex_weekly_percent: 0,
  last_codex_weekly_reset_at: 0,
  last_claude_fable_percent: 0,
  last_claude_fable_reset_at: 0,
  last_xai_weekly_percent: 0,
  last_xai_weekly_period_start_at: 0,
  last_xai_weekly_period_end_at: 0,
  last_xai_product_usage: '',
  last_xai_on_demand_cap: 0,
  last_xai_on_demand_used: 0,
  last_xai_billing_period_end_at: 0,
  last_error: '',
  last_codex_credits:
    '{"has_credits":true,"unlimited":false,"balance":"59897.56999999999"}',
  created_at: 0,
  updated_at: 0,
}

const labels = {
  fiveHour: 'Five-hour Limit',
  weekly: 'Weekly Limit',
  fable: 'Fable Limit',
  codexFiveHour: 'Codex Five-hour Limit',
  codexWeekly: 'Codex Weekly Limit',
  reset: 'Reset',
  quota: 'Quota',
}

test('悬浮用量显示取整的 Codex 积分及刷新错误，不展示精确余额', async () => {
  const user = userEvent.setup()
  render(
    <BindingUsageCell
      binding={{ ...binding, last_error: 'Usage refresh failed' }}
      labels={labels}
    />
  )
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  await user.hover(screen.getByLabelText('Usage'))
  const tooltip = await screen.findByRole('tooltip')
  expect(
    within(tooltip).getByRole('region', { name: 'Codex credits' })
  ).toBeVisible()
  expect(within(tooltip).getByText('59,897')).toBeVisible()
  expect(within(tooltip).queryByText('Exact balance')).not.toBeInTheDocument()
  expect(
    within(tooltip).queryByText('59897.56999999999')
  ).not.toBeInTheDocument()
  expect(within(tooltip).getByText('Usage refresh failed')).toBeVisible()
})

test('键盘聚焦用量打开积分提示，Escape 关闭', async () => {
  const user = userEvent.setup()
  render(<BindingUsageCell binding={binding} labels={labels} />)
  await user.tab()
  expect(screen.getByLabelText('Usage')).toHaveFocus()
  const tooltip = await screen.findByRole('tooltip')
  expect(within(tooltip).getByText('Credits balance')).toBeVisible()
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
})

test.each([
  [
    '{"has_credits":false,"unlimited":false,"balance":0}',
    '0',
    'No credits available',
  ],
  [
    '{"has_credits":true,"unlimited":true,"balance":null}',
    'Unlimited',
    'Available',
  ],
  [
    '{"has_credits":true,"unlimited":false,"balance":null}',
    'Unknown',
    'Available',
  ],
  ['{"has_credits":true,"balance":"0.25"}', '<1', 'Available'],
])('积分状态 %s 显示真实余额和可用性', (credits, balance, status) => {
  render(<CodexCreditsDetails credits={credits} />)
  const region = screen.getByRole('region', { name: 'Codex credits' })
  expect(within(region).getByText(balance)).toBeInTheDocument()
  expect(within(region).getByText(status)).toBeInTheDocument()
  expect(within(region).queryByText('Exact balance')).not.toBeInTheDocument()
})

test.each(['', '{}', 'null', '[]', 'invalid JSON', '{"balance":"NaN"}'])(
  '缺失或无效积分 %s 不显示虚构余额',
  (credits) => {
    render(<CodexCreditsDetails credits={credits} />)
    expect(screen.queryByRole('region')).not.toBeInTheDocument()
  }
)

test.each([
  ['zhCN', '59,897'],
  ['zhTW', '59,897'],
  ['en', '59,897'],
  ['fr', '59\u202f897'],
  ['ru', '59\u00a0897'],
  ['ja', '59,897'],
  ['vi', '59.897'],
  ['invalid_locale', new Intl.NumberFormat(undefined).format(59897)],
])('积分按界面语言 %s 分组且不展示原始小数', async (language, balance) => {
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: language,
    fallbackLng: false,
    resources: { [language]: { translation: {} } },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <CodexCreditsDetails credits={binding.last_codex_credits} />
    </I18nextProvider>
  )
  expect(
    screen.getByText(balance, { normalizer: (text) => text })
  ).toBeInTheDocument()
  expect(screen.queryByText('59897.56999999999')).not.toBeInTheDocument()
})

test('切换界面语言立即更新积分数字格式', async () => {
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: false,
    resources: { en: { translation: {} }, vi: { translation: {} } },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <CodexCreditsDetails credits={binding.last_codex_credits} />
    </I18nextProvider>
  )
  expect(screen.getByText('59,897')).toBeInTheDocument()
  await act(async () => {
    await i18n.changeLanguage('vi')
  })
  expect(screen.getByText('59.897')).toBeInTheDocument()
  expect(screen.queryByText('59,897')).not.toBeInTheDocument()
})
