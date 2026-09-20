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

import type { UsageLog } from '../../data/schema'
import { DetailsDialog } from '../dialogs/details-dialog'

const requestBody = JSON.stringify({
  model: 'jev-latest',
  state: { request: '<img src=x onerror=alert(1)> Write a function.' },
  questions: { coding: { type: 'noul', instructions: 'Is this code?' } },
})
const responseBody = JSON.stringify({ answers: { coding: { noul: 0.9 } } })

function renderDetails(exchange?: object, isAdmin = true) {
  const log: UsageLog = {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 2,
    content: '',
    username: 'root',
    remark: '',
    token_name: 'test',
    model_name: 'jev-latest',
    quota: 7,
    prompt_tokens: 352,
    completion_tokens: 38,
    use_time: 1,
    is_stream: false,
    channel: 21,
    channel_name: 'TypeSafe',
    token_id: 1,
    group: 'default',
    ip: '',
    request_id: 'typesafe-request',
    upstream_request_id: '',
    other: JSON.stringify({
      request_path: '/v1/systemone',
      admin_info: {
        typesafe: [{ stage: 'before', parent_request_id: 'chat-request' }],
        typesafe_exchange: exchange,
      },
    }),
  }
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <DetailsDialog
        log={log}
        isAdmin={isAdmin}
        isRoot={isAdmin}
        open
        onOpenChange={() => {}}
      />
    </QueryClientProvider>
  )
}

describe('TypeSafe input and output in log details', () => {
  test('admins can switch payloads and copy the actual response without executing input markup', async () => {
    const user = userEvent.setup()
    const clipboard = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockResolvedValue()
    renderDetails({
      request: { body: requestBody },
      response: { body: responseBody },
    })
    await user.click(screen.getByText('TypeSafe input and output'))
    expect(screen.getByRole('tab', { name: 'Input' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Is this code?')
    expect(screen.getByRole('tabpanel')).toHaveTextContent(
      '<img src=x onerror=alert(1)>'
    )
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: 'Output' }))
    const panel = screen.getByRole('tabpanel')
    expect(panel).toHaveTextContent('0.9')
    await user.click(
      within(panel).getByRole('button', { name: 'Copy to clipboard' })
    )
    expect(clipboard).toHaveBeenCalledWith(responseBody)
    await user.click(screen.getByRole('tab', { name: 'Input' }))
    await user.keyboard('{ArrowRight}{Enter}')
    expect(screen.getByRole('tab', { name: 'Output' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  test('legacy logs explain that input and output were not recorded', async () => {
    const user = userEvent.setup()
    renderDetails()
    await user.click(screen.getByText('TypeSafe input and output'))
    expect(
      screen.getByText('No input/output was recorded for this log.')
    ).toBeInTheDocument()
  })

  test('truncated JSON remains readable and shows its limit', async () => {
    const user = userEvent.setup()
    renderDetails({
      request: { body: '{"state":"partial', truncated: true },
      response: { body: responseBody },
    })
    await user.click(screen.getByText('TypeSafe input and output'))
    expect(screen.getByRole('tabpanel')).toHaveTextContent('{"state":"partial')
    expect(
      screen.getByText(
        'Only part of this body was saved because it exceeded the log limit.'
      )
    ).toBeInTheDocument()
  })

  test('non-admin views show sanitized TypeSafe answers', () => {
    const log: UsageLog = {
      id: 2,
      user_id: 1,
      created_at: 1,
      type: 2,
      content: '',
      username: 'user',
      remark: '',
      token_name: 'test',
      model_name: 'gpt-4.1',
      quota: 1,
      prompt_tokens: 1,
      completion_tokens: 1,
      use_time: 1,
      is_stream: false,
      channel: 1,
      channel_name: '',
      token_id: 1,
      group: 'default',
      ip: '',
      request_id: 'chat-request',
      upstream_request_id: '',
      other: JSON.stringify({
        typesafe: [
          {
            stage: 'before',
            status: 'success',
            answers: { category: { type: 'choice', choice: 'coding' } },
          },
        ],
      }),
    }
    render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <DetailsDialog
          log={log}
          isAdmin={false}
          isRoot={false}
          open
          onOpenChange={() => {}}
        />
      </QueryClientProvider>
    )
    expect(screen.getByText('TypeSafe evaluations')).toBeInTheDocument()
    expect(screen.getByText('coding')).toBeInTheDocument()
    expect(
      screen.queryByText('TypeSafe input and output')
    ).not.toBeInTheDocument()
  })

  test('non-admin views never show TypeSafe payloads', () => {
    renderDetails(
      { request: { body: requestBody }, response: { body: responseBody } },
      false
    )
    expect(screen.queryByRole('tab', { name: 'Input' })).not.toBeInTheDocument()
    expect(
      screen.queryByText('TypeSafe input and output')
    ).not.toBeInTheDocument()
    expect(document.body).not.toHaveTextContent('Is this code?')
  })
})
