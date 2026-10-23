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
            '[{"bucket_id":"gemini-5h","remaining_fraction":0.9804,"reset_at":0},{"bucket_id":"gemini-weekly","remaining_fraction":1,"reset_at":0}]',
        }}
      />
    )
    expect(screen.getByText('Pro')).toBeInTheDocument()
    expect(screen.getByText('1.96%')).toBeInTheDocument()
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
    expect(screen.getByText('0.14%')).toBeInTheDocument()
    expect(screen.getByText('0.83%')).toBeInTheDocument()
    expect(screen.queryByText('Claude and GPT models')).not.toBeInTheDocument()
    expect(screen.queryByText(/^Reset:/)).not.toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(2)

    await user.hover(screen.getByRole('button', { name: 'Antigravity' }))
    const details = within(await screen.findByRole('tooltip'))
    const thirdParty = within(
      details.getByRole('region', { name: 'Claude and GPT models' })
    )
    expect(thirdParty.getByText('0%')).toBeInTheDocument()
    expect(thirdParty.getByText('100%')).toBeInTheDocument()
    expect(details.getAllByText(/^Reset:/)).toHaveLength(4)
  })

  test.each([
    { bucketID: 'gemini-5h', label: 'Gemini Models 5-Hour Window Used' },
    {
      bucketID: '3p-weekly',
      label: 'Claude and GPT models Weekly Window Used',
    },
  ])(
    '上游仅返回 $bucketID 时展示该窗口，不补造其他窗口',
    ({ bucketID, label }) => {
      render(
        <AntigravityUsageCell
          binding={{
            last_error: '',
            last_antigravity_quota: JSON.stringify([
              { bucket_id: bucketID, remaining_fraction: 0.75, reset_at: 0 },
            ]),
          }}
        />
      )
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.getAllByRole('progressbar')).toHaveLength(1)
      expect(screen.getByRole('progressbar', { name: label })).toHaveAttribute(
        'aria-valuenow',
        '25'
      )
    }
  )

  test('合并 Gemini 共享额度和第三方窗口，不将模型额度伪装成五小时或周额度', async () => {
    const user = userEvent.setup()
    render(
      <AntigravityUsageCell
        binding={{
          last_error: '',
          last_plan_type: 'Google AI Pro',
          last_antigravity_quota: JSON.stringify([
            {
              bucket_id: 'gemini-shared',
              remaining_fraction: 0.75,
              reset_at: 1792752183,
            },
            { bucket_id: '3p-5h', remaining_fraction: 0, reset_at: 1792743914 },
            {
              bucket_id: '3p-weekly',
              remaining_fraction: 1,
              reset_at: 1793328283,
            },
          ]),
        }}
      />
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(1)
    expect(
      screen.getByRole('progressbar', {
        name: 'Gemini Models Shared quota Used',
      })
    ).toHaveAttribute('aria-valuenow', '25')
    await user.hover(screen.getByRole('button', { name: 'Antigravity' }))
    const details = within(await screen.findByRole('tooltip'))
    const gemini = within(
      details.getByRole('region', { name: 'Gemini Models' })
    )
    expect(gemini.getByText('Shared quota')).toBeInTheDocument()
    expect(gemini.queryByText('5-Hour Window')).not.toBeInTheDocument()
    expect(gemini.queryByText('Weekly Window')).not.toBeInTheDocument()
    const thirdParty = within(
      details.getByRole('region', { name: 'Claude and GPT models' })
    )
    expect(thirdParty.getByText('5-Hour Window')).toBeInTheDocument()
    expect(thirdParty.getByText('Weekly Window')).toBeInTheDocument()
  })

  test('仅返回 Claude 和 GPT 额度时展示真实用量，悬浮详情不补造 Gemini 额度', async () => {
    const user = userEvent.setup()
    render(
      <AntigravityUsageCell
        binding={{
          last_error: '',
          last_plan_type: 'Google AI Pro',
          last_refreshed_at: 1,
          last_antigravity_quota: JSON.stringify([
            { bucket_id: '3p-5h', remaining_fraction: 0, reset_at: 1792743914 },
            {
              bucket_id: '3p-weekly',
              remaining_fraction: 1,
              reset_at: 1793328283,
            },
          ]),
        }}
      />
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByText('Pro')).toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(2)
    expect(
      screen.getByRole('progressbar', {
        name: 'Claude and GPT models 5-Hour Window Used',
      })
    ).toHaveAttribute('aria-valuenow', '100')
    expect(
      screen.getByRole('progressbar', {
        name: 'Claude and GPT models Weekly Window Used',
      })
    ).toHaveAttribute('aria-valuenow', '0')

    await user.hover(screen.getByRole('button', { name: 'Antigravity' }))
    const details = within(await screen.findByRole('tooltip'))
    expect(
      details.getByRole('region', { name: 'Claude and GPT models' })
    ).toBeInTheDocument()
    expect(
      details.queryByRole('region', { name: 'Gemini Models' })
    ).not.toBeInTheDocument()
    expect(details.getAllByText(/^Reset:/)).toHaveLength(2)
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
              {
                bucket_id: 'gemini-weekly',
                remaining_fraction: 0.65,
                reset_at: 0,
              },
            ]),
          }}
        />
      )
      expect(screen.getByText(`${used}%`)).toBeInTheDocument()
      const progress = screen.getByRole('progressbar', {
        name: 'Gemini Models 5-Hour Window Used',
      })
      expect(progress).toHaveAttribute('aria-valuenow', String(used))
      expect(progress).toHaveClass(
        `[&_[data-slot=progress-indicator]]:bg-${color}-500`
      )
    }
  )

  test.each([
    '[{"bucket_id":"gemini-5h","disabled":true,"remaining_fraction":1,"reset_at":0}]',
    '[{"bucket_id":"gemini-5h","reset_at":0}]',
    '[{"bucket_id":"gemini-5h","remaining_fraction":2,"reset_at":0}]',
    '[{"bucket_id":"future-5h","remaining_fraction":1,"reset_at":0}]',
    '[{"bucket_id":"3p-5h","remaining_fraction":1,"reset_at":0},{"bucket_id":"3p-5h","remaining_fraction":0,"reset_at":0}]',
    'invalid JSON',
    '[]',
  ])(
    '额度窗口无效或缺失时直接显示错误，不保留套餐或不可用占位: %s',
    (quota) => {
      render(
        <AntigravityUsageCell
          binding={{
            last_error: '',
            last_plan_type: 'Google AI Pro',
            last_antigravity_quota: quota,
          }}
        />
      )
      expect(screen.getByRole('alert')).toHaveTextContent('Error')
      expect(screen.queryByText('Unavailable')).not.toBeInTheDocument()
      expect(screen.queryByText('Pro')).not.toBeInTheDocument()
      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    }
  )

  test('刷新失败直接显示具体原因，不显示上次成功的额度和套餐', () => {
    const error =
      '刷新额度失败，上游状态码: 403; PERMISSION_DENIED; VALIDATION_REQUIRED; Verify your account to continue.'
    render(
      <AntigravityUsageCell
        binding={{
          last_error: error,
          last_plan_type: 'Google AI Pro',
          last_antigravity_quota:
            '[{"bucket_id":"gemini-5h","remaining_fraction":0.9804,"reset_at":0},{"bucket_id":"gemini-weekly","remaining_fraction":1,"reset_at":0}]',
        }}
      />
    )
    expect(screen.getByRole('alert')).toHaveTextContent(error)
    expect(screen.queryByText('No quota data')).not.toBeInTheDocument()
    expect(screen.queryByText('Pro')).not.toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  test('有成功刷新时间却没有额度时显示错误，未刷新时保持空状态', () => {
    const { rerender } = render(
      <AntigravityUsageCell binding={{ last_error: '' }} />
    )
    expect(screen.getByText('No quota data')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    rerender(
      <AntigravityUsageCell
        binding={{ last_error: '', last_refreshed_at: 1 }}
      />
    )
    expect(screen.getByRole('alert')).toHaveTextContent('Error')
    expect(screen.queryByText('No quota data')).not.toBeInTheDocument()
  })
})
