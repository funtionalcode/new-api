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
import { SSE } from 'sse.js'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { ERROR_MESSAGES } from '../../constants'
import { createStreamRequestController } from '../use-stream-request'

class ControlledXMLHttpRequest extends EventTarget {
  static HEADERS_RECEIVED = 2
  static instances: ControlledXMLHttpRequest[] = []
  readyState = 0
  status = 200
  responseText = ''

  constructor() {
    super()
    ControlledXMLHttpRequest.instances.push(this)
  }

  open() {}
  setRequestHeader() {}
  getAllResponseHeaders() {
    return 'content-type: text/event-stream'
  }
  send() {
    this.readyState = ControlledXMLHttpRequest.HEADERS_RECEIVED
    this.dispatchEvent(new Event('readystatechange'))
  }
  abort() {
    this.dispatchEvent(new Event('abort'))
  }
  append(data: string) {
    this.responseText += `data: ${data}\n\n`
    this.dispatchEvent(new Event('progress'))
  }
  finish() {
    this.dispatchEvent(new Event('load'))
  }
}

async function openStream() {
  const callbacks = {
    onUpdate: vi.fn(),
    onComplete: vi.fn(),
    onError: vi.fn(),
  }
  const setStreaming = vi.fn()
  const controller = createStreamRequestController({
    getHeaders: async () => ({}),
    createSource: () => new SSE('/pg/chat/completions', { start: false }),
    setStreaming,
  })
  await controller.send(
    { model: 'grok-4.6', messages: [], stream: true },
    {},
    callbacks
  )
  const xhr = ControlledXMLHttpRequest.instances[0]
  xhr.append(
    JSON.stringify({ choices: [{ delta: { content: 'partial answer' } }] })
  )
  return { callbacks, controller, setStreaming, xhr }
}

beforeEach(() => {
  ControlledXMLHttpRequest.instances = []
  vi.stubGlobal('XMLHttpRequest', ControlledXMLHttpRequest)
})

afterEach(() => vi.unstubAllGlobals())

describe('playground stream termination', () => {
  test('reports HTTP 200 EOF without DONE and keeps the received content', async () => {
    const { callbacks, setStreaming, xhr } = await openStream()

    xhr.finish()

    expect(callbacks.onUpdate).toHaveBeenCalledWith('content', 'partial answer')
    expect(callbacks.onError).toHaveBeenCalledExactlyOnceWith(
      ERROR_MESSAGES.CONNECTION_CLOSED,
      undefined
    )
    expect(callbacks.onComplete).not.toHaveBeenCalled()
    expect(setStreaming).toHaveBeenLastCalledWith(false)
  })

  test('reports a browser abort before DONE and stops the loading state', async () => {
    const { callbacks, setStreaming, xhr } = await openStream()

    xhr.abort()

    expect(callbacks.onError).toHaveBeenCalledExactlyOnceWith(
      ERROR_MESSAGES.CONNECTION_CLOSED,
      undefined
    )
    expect(callbacks.onComplete).not.toHaveBeenCalled()
    expect(setStreaming).toHaveBeenLastCalledWith(false)
  })

  test('completes normally when DONE arrives before the connection closes', async () => {
    const { callbacks, setStreaming, xhr } = await openStream()

    xhr.append('[DONE]')
    xhr.finish()

    expect(callbacks.onComplete).toHaveBeenCalledExactlyOnceWith()
    expect(callbacks.onError).not.toHaveBeenCalled()
    expect(setStreaming).toHaveBeenLastCalledWith(false)
  })

  test.each(['stop', 'dispose'] as const)(
    'does not report an interruption when the caller explicitly invokes %s',
    async (action) => {
      const { callbacks, controller, xhr } = await openStream()

      controller[action]()
      xhr.finish()

      expect(callbacks.onError).not.toHaveBeenCalled()
      expect(callbacks.onComplete).not.toHaveBeenCalled()
    }
  )
})
