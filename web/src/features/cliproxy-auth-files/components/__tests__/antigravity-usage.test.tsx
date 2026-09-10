import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'

import { AntigravityUsageCell } from '../antigravity-usage-cell'

describe('Antigravity 额度展示', () => {
  test('显示真实 Pro 套餐标签，并与 Codex 的倍率区分', () => {
    render(
      <AntigravityUsageCell
        binding={{
          last_plan_type: 'Google AI Pro',
          last_error: '',
          last_antigravity_quota:
            '[{"bucket_id":"gemini-5h","remaining_fraction":0.9804,"reset_at":0}]',
        }}
      />
    )
    expect(screen.getByText('Pro')).toBeInTheDocument()
    expect(screen.getByText('Used 1.96%')).toBeInTheDocument()
    expect(screen.queryByText('20x')).not.toBeInTheDocument()
  })

  test.each(['antigravity', 'oauth', ''])(
    '套餐尚未获取时不把 %s 显示为套餐',
    (plan) => {
      render(
        <AntigravityUsageCell
          binding={{ last_plan_type: plan, last_error: '' }}
        />
      )
      expect(screen.queryByText('Pro')).not.toBeInTheDocument()
      if (plan) expect(screen.queryByText(plan)).not.toBeInTheDocument()
    }
  )
  test('表格仅展示 Gemini 两个窗口，悬浮时显示四个窗口与重置时间', async () => {
    const user = userEvent.setup()
    render(
      <AntigravityUsageCell
        binding={{
          last_error: '',
          last_antigravity_quota: JSON.stringify([
            {
              bucket_id: 'gemini-weekly',
              remaining_fraction: 0.9986187,
              reset_at: 1789625751,
            },
            {
              bucket_id: 'gemini-5h',
              remaining_fraction: 0.9917125,
              reset_at: 1789038951,
            },
            {
              bucket_id: '3p-weekly',
              remaining_fraction: 1,
              reset_at: 1789626746,
            },
            { bucket_id: '3p-5h', remaining_fraction: 0, reset_at: 1789039946 },
          ]),
        }}
      />
    )
    expect(screen.getByText('Used 0.14%')).toBeInTheDocument()
    expect(screen.getByText('Used 0.83%')).toBeInTheDocument()
    expect(screen.queryByText('Claude and GPT models')).not.toBeInTheDocument()
    expect(screen.queryByText(/^Reset:/)).not.toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(2)

    await user.hover(screen.getByRole('button', { name: 'Antigravity' }))
    const details = within(await screen.findByRole('tooltip'))
    const thirdParty = within(
      details.getByRole('region', { name: 'Claude and GPT models' })
    )
    expect(thirdParty.getByText('Used 0%')).toBeInTheDocument()
    expect(thirdParty.getByText('Used 100%')).toBeInTheDocument()
    expect(details.getAllByText(/^Reset:/)).toHaveLength(4)
  })

  test.each([
    { remaining: 1, used: 0, color: 'emerald' },
    { remaining: 0.2, used: 80, color: 'amber' },
    { remaining: 0, used: 100, color: 'rose' },
  ])(
    '剩余 $remaining 时进度条显示已使用 $used% 并匹配告警颜色',
    ({ remaining, used, color }) => {
      render(
        <AntigravityUsageCell
          binding={{
            last_error: '',
            last_antigravity_quota: JSON.stringify([
              {
                bucket_id: 'gemini-5h',
                remaining_fraction: remaining,
                reset_at: 0,
              },
            ]),
          }}
        />
      )
      expect(screen.getByText(`Used ${used}%`)).toBeInTheDocument()
      const progress = screen.getByRole('progressbar', {
        name: 'Gemini Models 5-Hour Window Used',
      })
      expect(progress).toHaveAttribute('aria-valuenow', String(used))
      expect(progress).toHaveClass(
        `[&_[data-slot=progress-indicator]]:bg-${color}-500`
      )
    }
  )

  test('缺失和禁用窗口显示不可用，不伪造满额', () => {
    render(
      <AntigravityUsageCell
        binding={{
          last_error: '',
          last_antigravity_quota:
            '[{"bucket_id":"gemini-5h","disabled":true,"remaining_fraction":1,"reset_at":0}]',
        }}
      />
    )
    expect(screen.getAllByText('Unavailable')).toHaveLength(2)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  test('首次刷新失败显示错误标记，键盘聚焦后显示错误详情', async () => {
    const user = userEvent.setup()
    render(
      <AntigravityUsageCell binding={{ last_error: 'upstream unavailable' }} />
    )
    expect(screen.getByText('No quota data')).toBeInTheDocument()
    expect(screen.getByText('Error')).toBeInTheDocument()
    expect(screen.queryByText('upstream unavailable')).not.toBeInTheDocument()
    await user.tab()
    expect(await screen.findByRole('tooltip')).toHaveTextContent(
      'upstream unavailable'
    )
  })
})
