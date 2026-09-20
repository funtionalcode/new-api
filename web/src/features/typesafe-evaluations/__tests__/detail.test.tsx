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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { getTypeSafeEvaluationDetail } from '../api'
import { EvaluationDetailDialog } from '../components/evaluation-detail-dialog'
import type { TypeSafeEvaluationDetail } from '../types'

vi.mock('../api', () => ({ getTypeSafeEvaluationDetail: vi.fn() }))
beforeEach(() => vi.mocked(getTypeSafeEvaluationDetail).mockReset())

function renderDetails() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <EvaluationDetailDialog
        requestId='own-request'
        stage='before'
        onClose={vi.fn()}
      />
    </QueryClientProvider>
  )
}

test('shows loading until permission check finishes, then supports retry after denial', async () => {
  const user = userEvent.setup()
  const pending = Promise.withResolvers<TypeSafeEvaluationDetail>()
  vi.mocked(getTypeSafeEvaluationDetail).mockReturnValueOnce(pending.promise)
  renderDetails()
  expect(screen.getByRole('status')).toHaveTextContent('Loading')
  expect(screen.queryByRole('tab')).not.toBeInTheDocument()
  pending.reject(new Error('403'))
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Evaluation details are unavailable or you do not have permission.'
  )
  expect(screen.queryByRole('tab')).not.toBeInTheDocument()
  vi.mocked(getTypeSafeEvaluationDetail).mockResolvedValue({
    stage: 'before',
    model: 'jev-latest',
    status: 'success',
    request: {
      body: JSON.stringify({
        state: { request: 'Recovered evaluation input' },
      }),
    },
  })
  await user.click(screen.getByRole('button', { name: 'Retry' }))
  expect(await screen.findByText('Recovered evaluation input')).toBeVisible()
})

test('legacy logs show missing input and preserve saved evaluation answers as a fallback', async () => {
  const user = userEvent.setup()
  vi.mocked(getTypeSafeEvaluationDetail).mockResolvedValue({
    stage: 'before',
    status: 'success',
    answers: { category: { choice: 'coding', confidence: 0.98 } },
  })
  renderDetails()
  expect(
    await screen.findByText('No input/output was recorded for this log.')
  ).toBeVisible()
  await user.click(screen.getByRole('tab', { name: 'Evaluation result' }))
  expect(
    screen.getByText(
      'The original output was not recorded. Saved evaluation answers are shown.'
    )
  ).toBeVisible()
  expect(screen.getByRole('tabpanel')).toHaveTextContent('confidence')
  expect(screen.getByRole('tabpanel')).toHaveTextContent('0.98')
})

test('truncated malformed JSON remains readable, copyable and safely escaped', async () => {
  const user = userEvent.setup()
  const body = '{"state":{"request":"<img src=x onerror=alert(1)>'
  vi.mocked(getTypeSafeEvaluationDetail).mockResolvedValue({
    stage: 'before',
    status: 'success',
    truncated: true,
    request: { body, truncated: true },
  })
  renderDetails()
  expect(await screen.findByText(body)).toBeVisible()
  expect(
    screen.getByText(
      'Only part of this body was saved because it exceeded the log limit.'
    )
  ).toBeVisible()
  expect(screen.getByRole('alert')).toHaveTextContent(
    'The text sent for evaluation was truncated.'
  )
  expect(
    within(screen.getByRole('tabpanel')).queryByRole('img')
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Copy to clipboard' }))
  expect(await navigator.clipboard.readText()).toBe(body)
})
