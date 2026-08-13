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
import { describe, test } from 'node:test'

import type { ChatCompletionRequest } from '../types'
import { createStreamRequestController } from './use-stream-request'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((promiseResolve) => {
    resolve = promiseResolve
  })
  return { promise, resolve }
}

class FakeStreamSource {
  readyState = 0
  closed = false
  streamed = false
  private listeners = new Map<
    string,
    Array<
      (
        event: Event & {
          data?: string
          readyState?: number
          headers?: Record<string, string[]>
        }
      ) => void
    >
  >()

  addEventListener(
    type: string,
    listener: (
      event: Event & {
        data?: string
        readyState?: number
        headers?: Record<string, string[]>
      }
    ) => void
  ) {
    const listeners = this.listeners.get(type) ?? []
    listeners.push(listener)
    this.listeners.set(type, listeners)
  }

  close() {
    this.closed = true
  }

  stream() {
    this.streamed = true
  }

  emit(type: string, data?: string, headers?: Record<string, string[]>) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener({ data, readyState: this.readyState, headers } as Event & {
        data?: string
        readyState?: number
        headers?: Record<string, string[]>
      })
    }
  }
}

const payload: ChatCompletionRequest = {
  model: 'test-model',
  messages: [{ role: 'user', content: 'hello' }],
  stream: true,
}

const noopCallbacks = {
  onUpdate: () => undefined,
  onComplete: () => undefined,
  onError: () => undefined,
}

const noRequestHeaders: Record<string, string> = {}

describe('latest-wins stream request coordination', () => {
  test('only creates a stream for the latest header request', async () => {
    const firstHeaders = deferred<Record<string, string>>()
    const secondHeaders = deferred<Record<string, string>>()
    let headerRequest = 0
    const sources: FakeStreamSource[] = []
    const controller = createStreamRequestController({
      getHeaders: () => {
        headerRequest += 1
        return headerRequest === 1
          ? firstHeaders.promise
          : secondHeaders.promise
      },
      createSource: () => {
        const source = new FakeStreamSource()
        sources.push(source)
        return source
      },
      setStreaming: () => undefined,
    })

    const first = controller.send(payload, noRequestHeaders, noopCallbacks)
    const second = controller.send(payload, noRequestHeaders, noopCallbacks)
    firstHeaders.resolve({ Authorization: 'Bearer stale' })
    await first
    assert.equal(sources.length, 0)

    secondHeaders.resolve({ Authorization: 'Bearer current' })
    await second
    assert.equal(sources.length, 1)
    assert.equal(sources[0]?.streamed, true)
  })

  test('stop cancels a request that is still waiting for headers', async () => {
    const headers = deferred<Record<string, string>>()
    let sourceCount = 0
    const controller = createStreamRequestController({
      getHeaders: () => headers.promise,
      createSource: () => {
        sourceCount += 1
        return new FakeStreamSource()
      },
      setStreaming: () => undefined,
    })

    const request = controller.send(payload, noRequestHeaders, noopCallbacks)
    controller.stop()
    headers.resolve({ Authorization: 'Bearer ignored' })
    await request

    assert.equal(sourceCount, 0)
  })

  test('dispose cancels a pending header request without a state update', async () => {
    const headers = deferred<Record<string, string>>()
    const streamingStates: boolean[] = []
    let sourceCount = 0
    const controller = createStreamRequestController({
      getHeaders: () => headers.promise,
      createSource: () => {
        sourceCount += 1
        return new FakeStreamSource()
      },
      setStreaming: (streaming) => streamingStates.push(streaming),
    })

    const request = controller.send(payload, noRequestHeaders, noopCallbacks)
    controller.dispose()
    headers.resolve({ Authorization: 'Bearer ignored' })
    await request

    assert.equal(sourceCount, 0)
    assert.deepEqual(streamingStates, [false])
  })

  test('closes the previous source and ignores all of its later events', async () => {
    const nextHeaders = deferred<Record<string, string>>()
    let headerRequest = 0
    const sources: FakeStreamSource[] = []
    const updates: string[] = []
    const controller = createStreamRequestController({
      getHeaders: () => {
        headerRequest += 1
        if (headerRequest === 1) {
          return Promise.resolve({ Authorization: 'Bearer first' })
        }
        return nextHeaders.promise
      },
      createSource: () => {
        const source = new FakeStreamSource()
        sources.push(source)
        return source
      },
      setStreaming: () => undefined,
    })
    const callbacks = {
      onUpdate: (_type: 'reasoning' | 'content', chunk: string) =>
        updates.push(chunk),
      onComplete: () => undefined,
      onError: () => undefined,
    }

    await controller.send(payload, noRequestHeaders, callbacks)
    const second = controller.send(payload, noRequestHeaders, callbacks)
    assert.equal(sources[0]?.closed, true)
    sources[0]?.emit(
      'message',
      JSON.stringify({ choices: [{ delta: { content: 'stale' } }] })
    )

    nextHeaders.resolve({ Authorization: 'Bearer second' })
    await second
    sources[1]?.emit(
      'message',
      JSON.stringify({ choices: [{ delta: { content: 'current' } }] })
    )

    assert.deepEqual(updates, ['current'])
  })

  test('merges Cursor session headers and exposes response headers on open', async () => {
    let sourceHeaders: Record<string, string> = {}
    const source = new FakeStreamSource()
    const openedHeaders: Array<Record<string, string[]>> = []
    const controller = createStreamRequestController({
      getHeaders: () => Promise.resolve({ Authorization: 'Bearer user' }),
      createSource: (_payload, headers) => {
        sourceHeaders = headers
        return source
      },
      setStreaming: () => undefined,
    })

    await controller.send(
      payload,
      { 'X-Cursor-Persistent': 'true' },
      { ...noopCallbacks, onOpen: (headers) => openedHeaders.push(headers) }
    )
    source.emit('open', undefined, {
      'x-cursor-agent-id': ['bc-00000000-0000-0000-0000-000000000001'],
    })

    assert.deepEqual(sourceHeaders, {
      Authorization: 'Bearer user',
      'X-Cursor-Persistent': 'true',
    })
    assert.equal(
      openedHeaders[0]?.['x-cursor-agent-id']?.[0],
      'bc-00000000-0000-0000-0000-000000000001'
    )
  })
})
