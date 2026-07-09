import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { createUserMessage, formatMessageForAPI } from './message-utils'

describe('playground message utils', () => {
  test('formats user screenshots as image_url content parts', () => {
    const message = createUserMessage(
      'describe this screenshot',
      1783566400,
      'chat',
      ['data:image/png;base64,abc']
    )

    const formatted = formatMessageForAPI(message)

    assert.equal(formatted.role, 'user')
    assert.deepEqual(formatted.content, [
      { type: 'text', text: 'describe this screenshot' },
      {
        type: 'image_url',
        image_url: { url: 'data:image/png;base64,abc' },
      },
    ])
  })
})
