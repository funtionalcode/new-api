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
import assert from 'node:assert/strict'

import { describe, expect, test } from 'vitest'

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../../../constants'
import type { Message } from '../../../types'
import { buildChatCompletionPayload } from '../payload-builder'

const messages: Message[] = [
  {
    key: 'user-1',
    from: 'user',
    versions: [{ id: 'user-1-v1', content: 'continue until complete' }],
  },
]

describe('playground chat completion payload', () => {
  test.each([
    'claude-sonnet-5',
    'claude-sonnet-5-high',
    'claude-sonnet-5-thinking',
  ])(
    'uses the Sonnet 5 output limit for %s when max tokens is automatic',
    (model) => {
      const payload = buildChatCompletionPayload(
        messages,
        { ...DEFAULT_CONFIG, model },
        { ...DEFAULT_PARAMETER_ENABLED, max_tokens: false }
      )

      expect(payload.max_tokens).toBe(128000)
    }
  )

  test.each([0, 8192, 32768])(
    'preserves the explicit Sonnet 5 max tokens limit of %i',
    (maxTokens) => {
      const payload = buildChatCompletionPayload(
        messages,
        { ...DEFAULT_CONFIG, model: 'claude-sonnet-5', max_tokens: maxTokens },
        { ...DEFAULT_PARAMETER_ENABLED, max_tokens: true }
      )

      expect(payload.max_tokens).toBe(maxTokens)
    }
  )

  test.each(['claude-sonnet-50', 'claude-sonnet-4-6', 'claude-3-5-sonnet'])(
    'does not apply the Sonnet 5 output limit to %s',
    (model) => {
      const payload = buildChatCompletionPayload(
        messages,
        { ...DEFAULT_CONFIG, model },
        { ...DEFAULT_PARAMETER_ENABLED, max_tokens: false }
      )

      expect(payload.max_tokens).toBeUndefined()
    }
  )

  test('uses the GLM 5.3 long-output default when max tokens is automatic', () => {
    const payload = buildChatCompletionPayload(
      messages,
      { ...DEFAULT_CONFIG, model: 'glm-5.3' },
      { ...DEFAULT_PARAMETER_ENABLED, max_tokens: false }
    )

    assert.equal(payload.max_tokens, 65536)
  })

  test('uses the GLM 5.3 long-output default for model variants', () => {
    const payload = buildChatCompletionPayload(
      messages,
      { ...DEFAULT_CONFIG, model: 'glm-5.3-flash' },
      { ...DEFAULT_PARAMETER_ENABLED, max_tokens: false }
    )

    assert.equal(payload.max_tokens, 65536)
  })

  test('preserves an explicit max tokens limit for GLM 5.3', () => {
    const payload = buildChatCompletionPayload(
      messages,
      { ...DEFAULT_CONFIG, model: 'glm-5.3', max_tokens: 8192 },
      { ...DEFAULT_PARAMETER_ENABLED, max_tokens: true }
    )

    assert.equal(payload.max_tokens, 8192)
  })

  test('keeps automatic max tokens omitted for unrelated models', () => {
    const payload = buildChatCompletionPayload(
      messages,
      { ...DEFAULT_CONFIG, model: 'gpt-4o' },
      { ...DEFAULT_PARAMETER_ENABLED, max_tokens: false }
    )

    assert.equal(payload.max_tokens, undefined)
  })

  test('does not treat a similarly prefixed model as a GLM 5.3 variant', () => {
    const payload = buildChatCompletionPayload(
      messages,
      { ...DEFAULT_CONFIG, model: 'glm-5.30' },
      { ...DEFAULT_PARAMETER_ENABLED, max_tokens: false }
    )

    assert.equal(payload.max_tokens, undefined)
  })
})
