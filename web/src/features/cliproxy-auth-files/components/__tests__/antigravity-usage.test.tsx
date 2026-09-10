import { render, screen, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { AntigravityUsageCell } from '../antigravity-usage-cell'

describe('Antigravity 额度展示', () => {
  test('按模型组区分四个窗口并保留剩余百分比小数', () => {
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
    const gemini = within(screen.getByRole('region', { name: 'Gemini Models' }))
    expect(gemini.getByText('Remaining 99.86%')).toBeInTheDocument()
    expect(gemini.getByText('Remaining 99.17%')).toBeInTheDocument()
    const thirdParty = within(
      screen.getByRole('region', { name: 'Claude and GPT models' })
    )
    expect(thirdParty.getByText('Remaining 100%')).toBeInTheDocument()
    expect(thirdParty.getByText('Remaining 0%')).toBeInTheDocument()
    expect(screen.getAllByText(/^Reset:/)).toHaveLength(4)
    expect(screen.getAllByRole('progressbar')).toHaveLength(4)
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
    expect(screen.getAllByText('Unavailable')).toHaveLength(4)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  test('首次刷新失败展示空状态和错误', () => {
    render(
      <AntigravityUsageCell binding={{ last_error: 'upstream unavailable' }} />
    )
    expect(screen.getByText('No quota data')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('upstream unavailable')
  })
})
