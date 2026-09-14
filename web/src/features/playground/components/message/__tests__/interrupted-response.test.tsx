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
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { sanitizeMessagesOnLoad } from '../../../lib/message/message-streaming-utils'
import { updateAssistantMessageWithError } from '../../../lib/message/message-update-utils'
import { getMessageContent } from '../../../lib/message/message-utils'
import { messagesSchema } from '../../../lib/storage/storage-schema'
import type { Message } from '../../../types'
import { PlaygroundMessageContent } from '../playground-message-content'

const partial: Message = {
  key: 'partial-response',
  from: 'assistant',
  status: 'streaming',
  versions: [{ id: 'v1', content: 'The generated part of the answer.' }],
}

function renderMessage(message: Message) {
  return render(
    <PlaygroundMessageContent
      actions={<button type='button'>Copy answer</button>}
      alignment='left'
      errorActions={<button type='button'>Retry request</button>}
      message={message}
      versionContent={getMessageContent(message)}
    />
  )
}

describe('interrupted playground responses', () => {
  test('keeps the partial answer visible alongside the connection error', () => {
    const message = updateAssistantMessageWithError(
      [partial],
      'Connection closed'
    )[0]

    renderMessage(message)

    expect(
      screen.getByText('The generated part of the answer.')
    ).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('Connection closed')
    expect(
      screen.getByRole('button', { name: 'Retry request' })
    ).toBeInTheDocument()
  })

  test('retains the answer and separate error through a storage round trip', () => {
    const messages = updateAssistantMessageWithError(
      [partial],
      'Connection closed'
    )
    const serialized = JSON.stringify(messages)
    const restored = messagesSchema.parse(JSON.parse(serialized))

    renderMessage(restored[0])

    expect(
      screen.getByText('The generated part of the answer.')
    ).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('Connection closed')
  })

  test('marks a saved unfinished answer as interrupted when the page is reopened', () => {
    const restored = sanitizeMessagesOnLoad([partial])[0]

    renderMessage(restored)

    expect(restored.status).toBe('error')
    expect(
      screen.getByText('The generated part of the answer.')
    ).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Generation was interrupted'
    )
  })

  test('shows only the error when no answer was generated', () => {
    const empty = { ...partial, versions: [{ id: 'v1', content: '' }] }
    const message = updateAssistantMessageWithError(
      [empty],
      'Connection closed'
    )[0]

    renderMessage(message)

    expect(screen.getAllByText(/Connection closed/)).toHaveLength(1)
    expect(
      screen.queryByText('The generated part of the answer.')
    ).not.toBeInTheDocument()
  })

  test('keeps a saved completed answer complete when reopening the page', () => {
    const completed: Message = { ...partial, status: 'complete' }
    const restored = sanitizeMessagesOnLoad([completed])[0]

    renderMessage(restored)

    expect(restored.status).toBe('complete')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
