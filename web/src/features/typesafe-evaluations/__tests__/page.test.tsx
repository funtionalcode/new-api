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
import { render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { getTypeSafeEvaluations } from '../api'
import { TypeSafeEvaluationsPage } from '../index'

vi.mock('../api', () => ({
  getTypeSafeEvaluations: vi.fn(),
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
              status: 'success',
              answers: { category: { type: 'choice', choice: 'coding' } },
            },
            after: {
              stage: 'after',
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
    expect(screen.getByText('gpt-4.1')).toBeInTheDocument()
    expect(screen.queryByText('private input')).not.toBeInTheDocument()
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
