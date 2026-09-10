import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'

import { AntigravityUsageCell } from '../antigravity-usage-cell'

describe('Antigravity 额度展示', () => {
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
    expect(screen.getByText('Remaining 99.86%')).toBeInTheDocument()
    expect(screen.getByText('Remaining 99.17%')).toBeInTheDocument()
    expect(screen.queryByText('Claude and GPT models')).not.toBeInTheDocument()
    expect(screen.queryByText(/^Reset:/)).not.toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(2)

    await user.hover(screen.getByRole('button', { name: 'Antigravity' }))
    const details = within(await screen.findByRole('tooltip'))
    const thirdParty = within(
      details.getByRole('region', { name: 'Claude and GPT models' })
    )
    expect(thirdParty.getByText('Remaining 100%')).toBeInTheDocument()
    expect(thirdParty.getByText('Remaining 0%')).toBeInTheDocument()
    expect(details.getAllByText(/^Reset:/)).toHaveLength(4)
  })

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
