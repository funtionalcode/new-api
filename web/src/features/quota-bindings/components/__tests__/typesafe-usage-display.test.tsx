import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { TypeSafeUsageBinding } from '../../types'
import { TypeSafeUsageCells } from '../typesafe-usage-cells'

const binding: TypeSafeUsageBinding = {
  id: 1,
  name: 'TypeSafe account',
  note: '',
  enabled: true,
  has_curl: true,
  last_refreshed_at: 0,
  last_error: '',
  created_at: 0,
  updated_at: 0,
  last_buckets: '',
}

describe('TypeSafe usage details', () => {
  it('disables usage details until the first successful refresh', () => {
    render(
      <table>
        <tbody>
          <tr>
            <TypeSafeUsageCells binding={binding} />
          </tr>
        </tbody>
      </table>
    )
    expect(screen.getByRole('button', { name: 'View Usage' })).toBeDisabled()
  })
  it('opens an empty snapshot and exposes accessible filters', () => {
    render(
      <table>
        <tbody>
          <tr>
            <TypeSafeUsageCells
              binding={{ ...binding, last_refreshed_at: 1, last_buckets: '[]' }}
            />
          </tr>
        </tbody>
      </table>
    )
    fireEvent.click(screen.getByRole('button', { name: 'View Usage' }))
    expect(screen.getByRole('dialog')).toHaveTextContent('TypeSafe account')
    expect(screen.getByText('No Data')).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Time Range' }), {
      target: { value: '30' },
    })
    expect(screen.getByRole('combobox', { name: 'Time Range' })).toHaveValue(
      '30'
    )
    fireEvent.change(screen.getByRole('combobox', { name: 'Traffic' }), {
      target: { value: 'playground' },
    })
    expect(screen.getByRole('combobox', { name: 'Traffic' })).toHaveValue(
      'playground'
    )
    expect(screen.getByRole('combobox', { name: 'Granularity' })).toHaveValue(
      'hour'
    )
  })
  it('updates visible totals when traffic changes and supports daily charts', () => {
    const day = new Date().toISOString()
    const last_buckets = JSON.stringify([
      {
        day,
        apiKeyId: 'key-a',
        apiKeyName: 'Test key',
        userId: null,
        inputTokens: 100,
        outputTokens: 23,
        requests: 4,
      },
      {
        day,
        apiKeyId: null,
        apiKeyName: null,
        userId: 'user-a',
        inputTokens: 50,
        outputTokens: 6,
        requests: 2,
      },
    ])
    render(
      <table>
        <tbody>
          <tr>
            <TypeSafeUsageCells
              binding={{ ...binding, last_refreshed_at: 1, last_buckets }}
            />
          </tr>
        </tbody>
      </table>
    )
    fireEvent.click(screen.getByRole('button', { name: 'View Usage' }))
    expect(screen.getByText('179')).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Traffic' }), {
      target: { value: 'key:key-a' },
    })
    expect(screen.getByText('123')).toBeInTheDocument()
    expect(screen.queryByText('179')).not.toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Granularity' }), {
      target: { value: 'day' },
    })
    expect(screen.getByRole('combobox', { name: 'Granularity' })).toHaveValue(
      'day'
    )
    expect(screen.getByText('123')).toBeInTheDocument()
  })
})
