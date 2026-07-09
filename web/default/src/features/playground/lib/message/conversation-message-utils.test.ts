import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { MESSAGE_ROLES, MESSAGE_STATUS } from '../../constants'
import type { Message } from '../../types'
import {
  createRegeneratedMessages,
  getPendingGenerationMode,
} from './conversation-message-utils'
import { createUserMessage } from './message-utils'

describe('playground conversation message utils', () => {
  test('regenerates legacy task errors with the previous user message mode', () => {
    const userMessage = createUserMessage(
      'make a video',
      1783566400,
      'video'
    )
    const legacyErrorMessage: Message = {
      key: 'assistant-error',
      from: MESSAGE_ROLES.ASSISTANT,
      versions: [{ id: 'error', content: 'Request error occurred' }],
      status: MESSAGE_STATUS.ERROR,
    }

    const messages = createRegeneratedMessages(
      [userMessage, legacyErrorMessage],
      legacyErrorMessage.key
    )

    assert.equal(messages?.at(-1)?.mode, 'video')
    assert.equal(getPendingGenerationMode(messages ?? []), 'video')
  })
})
