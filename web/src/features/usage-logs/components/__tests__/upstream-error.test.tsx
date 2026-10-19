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
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { DetailsDialog } from '../dialogs/details-dialog'

const upstreamError =
  'status_code=500, Our servers are currently overloaded. Please try again later.'
const publicError = 'status_code=500, Internal server error'
const queryClients: QueryClient[] = []

function renderDetails(
  other: Record<string, unknown>,
  isAdmin = true,
  isRoot = false
): void {
  const log: UsageLog = {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 5,
    content: publicError,
    username: 'user',
    remark: '',
    token_name: 'token',
    model_name: 'gpt-test',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 9,
    is_stream: true,
    channel: 4,
    channel_name: 'channel',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: 'req-1',
    upstream_request_id: '',
  }
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['status'], {}, { updatedAt: Date.now() + 60_000 })
  queryClients.push(queryClient)

  render(
    <QueryClientProvider client={queryClient}>
      <DetailsDialog
        log={log}
        isAdmin={isAdmin}
        isRoot={isRoot}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
}

afterEach(() => {
  for (const queryClient of queryClients) queryClient.clear()
  queryClients.length = 0
})

describe('usage log upstream error', () => {
  test.each([false, true])(
    'shows the upstream error separately from public content for admin with root=%s',
    (isRoot) => {
      renderDetails(
        { admin_info: { upstream_error: upstreamError } },
        true,
        isRoot
      )

      expect(screen.getByText('Upstream Error')).toBeVisible()
      expect(screen.getByText(upstreamError)).toBeVisible()
      expect(screen.getByText(publicError)).toBeVisible()
    }
  )

  test('hides upstream error content and its copy action from non-admins even if metadata is supplied', () => {
    renderDetails({ admin_info: { upstream_error: upstreamError } }, false)

    expect(screen.queryByText('Upstream Error')).not.toBeInTheDocument()
    expect(screen.queryByText(upstreamError)).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Copy Upstream Error' })
    ).not.toBeInTheDocument()
    expect(screen.getByText(publicError)).toBeVisible()
  })

  test.each([undefined, '', ' \n ', { message: upstreamError }])(
    'omits the upstream error section for missing, empty or invalid field %j',
    (value) => {
      renderDetails({ admin_info: { upstream_error: value } })

      expect(screen.queryByText('Upstream Error')).not.toBeInTheDocument()
      expect(screen.getByText(publicError)).toBeVisible()
    }
  )

  test('copies the complete upstream error without replacing or trimming it', async () => {
    const user = userEvent.setup()
    const writeText = vi.spyOn(navigator.clipboard, 'writeText')
    const multilineError = `${upstreamError}\nupstream_request_id=req-upstream-1\n`
    renderDetails({ admin_info: { upstream_error: multilineError } })

    await user.click(
      screen.getByRole('button', { name: 'Copy Upstream Error' })
    )

    expect(writeText).toHaveBeenCalledWith(multilineError)
    expect(screen.getByRole('button', { name: 'Copied' })).toBeVisible()
  })
})
