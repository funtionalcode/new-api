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
import { describe, expect, test, vi } from 'vitest'

import { getTypeSafeEvaluationDetail, getTypeSafeEvaluations } from '../api'
import { TypeSafeEvaluationsPage } from '../index'

vi.mock('../api', () => ({
  getTypeSafeEvaluations: vi.fn(),
  getTypeSafeEvaluationDetail: vi.fn(),
}))

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <TypeSafeEvaluationsPage />
    </QueryClientProvider>
  )
}

describe('TypeSafe evaluations page', () => {
  test('opens stage details on demand and exposes evaluated text, questions and full output', async () => {
    const user = userEvent.setup()
    vi.mocked(getTypeSafeEvaluationDetail).mockClear()
    vi.mocked(getTypeSafeEvaluationDetail).mockResolvedValue({
      stage: 'after',
      model: 'jev-latest',
      status: 'success',
      truncated: true,
      request: {
        body: JSON.stringify({
          state: { request: 'Write a greeting', response: 'Hello world' },
          questions: {
            relevance: {
              type: 'score',
              instructions: 'Does the answer address the request?',
              criteria: ['No', 'Yes'],
            },
          },
        }),
      },
      response: {
        body: JSON.stringify({
          answers: {
            relevance: {
              score: 1.99,
              confidence: 0.98,
              probabilities: [0.01, 0.99],
            },
          },
        }),
      },
    })
    vi.mocked(getTypeSafeEvaluations).mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            created_at: 1710000000,
            model_name: 'grok-4.6',
            token_name: 'canary',
            request_id: 'req-detail',
            after: {
              model: 'jev-latest',
              status: 'success',
              answers: { relevance: { score: 1.99 } },
            },
          },
        ],
      },
    })
    renderPage()
    const button = await screen.findByRole('button', {
      name: 'View evaluation details',
    })
    expect(getTypeSafeEvaluationDetail).not.toHaveBeenCalled()
    button.focus()
    await user.keyboard('{Enter}')
    const dialog = await screen.findByRole('dialog', {
      name: 'After evaluation details',
    })
    expect(await within(dialog).findByText('Write a greeting')).toBeVisible()
    expect(within(dialog).getByText('Hello world')).toBeVisible()
    expect(getTypeSafeEvaluationDetail).toHaveBeenCalledWith(
      'req-detail',
      'after'
    )
    const contentTab = within(dialog).getByRole('tab', {
      name: 'Evaluation content',
    })
    contentTab.focus()
    await user.keyboard('{ArrowRight}{Enter}')
    expect(within(dialog).getByRole('tabpanel')).toHaveTextContent(
      'Does the answer address the request?'
    )
    await user.click(
      within(dialog).getByRole('tab', { name: 'Evaluation result' })
    )
    expect(within(dialog).getByRole('tabpanel')).toHaveTextContent('confidence')
    expect(within(dialog).getByRole('tabpanel')).toHaveTextContent('0.98')
    await user.click(
      within(dialog).getByRole('button', { name: 'Copy to clipboard' })
    )
    expect(await navigator.clipboard.readText()).toContain('probabilities')
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  test('shows before and after answers for each question', async () => {
    vi.mocked(getTypeSafeEvaluations).mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            created_at: 1710000000,
            model_name: 'gpt-4.1',
            token_name: 'canary',
            request_id: 'req-1',
            before: {
              stage: 'before',
              model: 'jev-latest',
              status: 'success',
              answers: { category: { type: 'choice', choice: 'coding' } },
            },
            after: {
              stage: 'after',
              model: 'jev-latest',
              status: 'success',
              answers: { relevance: { type: 'score', score: 2 } },
            },
          },
        ],
      },
    })

    renderPage()

    expect(await screen.findByText('coding')).toBeInTheDocument()
    expect(screen.getByText('relevance')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('Evaluation model: jev-latest')).toBeInTheDocument()
    expect(screen.getByText('Evaluated model: gpt-4.1')).toBeInTheDocument()
    expect(screen.queryByText('private input')).not.toBeInTheDocument()
  })

  test('permission failures explain that no evaluation ran and keep the evaluator distinct', async () => {
    vi.mocked(getTypeSafeEvaluations).mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            created_at: 1710000000,
            model_name: 'grok-4.6',
            token_name: 'canary',
            request_id: 'req-denied',
            before: {
              model: 'jev-latest',
              status: 'error',
              reason: 'channel_unavailable_or_forbidden',
            },
            after: { status: 'skipped', reason: 'no_text_output' },
          },
        ],
      },
    })
    renderPage()
    expect(
      await screen.findByText('Evaluation model: jev-latest')
    ).toBeInTheDocument()
    expect(screen.getByText('Evaluated model: grok-4.6')).toBeInTheDocument()
    expect(screen.getByText('Not evaluated')).toBeInTheDocument()
    expect(
      screen.getByText(
        'The evaluation channel is unavailable or you do not have access. No evaluation was run.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText('No answer text was available for evaluation.')
    ).toBeInTheDocument()
  })

  test('missing evaluator metadata is not inferred from the main model', async () => {
    vi.mocked(getTypeSafeEvaluations).mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            created_at: 1710000000,
            model_name: 'grok-4.6',
            token_name: '',
            request_id: 'req-legacy',
            before: {
              status: 'success',
              answers: { coding: { type: 'noul', noul: 0.95 } },
            },
          },
        ],
      },
    })
    renderPage()
    expect(
      await screen.findByText('Evaluation model not recorded')
    ).toBeInTheDocument()
    expect(screen.getByText('Evaluated model: grok-4.6')).toBeInTheDocument()
    expect(screen.getByText('0.95')).toBeInTheDocument()
  })

  test('shows an empty state when there are no evaluations', async () => {
    vi.mocked(getTypeSafeEvaluations).mockResolvedValue({
      success: true,
      data: { page: 1, page_size: 20, total: 0, items: [] },
    })

    renderPage()

    expect(
      await screen.findByText('No TypeSafe evaluations')
    ).toBeInTheDocument()
  })
})
