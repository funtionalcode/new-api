import assert from 'node:assert/strict'
import { afterEach, beforeEach, describe, test } from 'node:test'

import type { Message } from '../../types'
import {
  clearCursorAgentSession,
  clearPlaygroundData,
  loadCursorAgentSession,
  loadMessages,
  saveCursorAgentSession,
  saveMessages,
} from './storage'

class LocalStorageMock {
  private store = new Map<string, string>()
  throwOnSet = false

  getItem(key: string) {
    return this.store.get(key) ?? null
  }

  setItem(key: string, value: string) {
    if (this.throwOnSet) {
      throw new Error('quota exceeded')
    }
    this.store.set(key, value)
  }

  removeItem(key: string) {
    this.store.delete(key)
  }

  clear() {
    this.store.clear()
  }
}

function createImageMessage(content: string): Message {
  return {
    key: `assistant-${Date.now()}`,
    from: 'assistant',
    mode: 'image',
    status: 'complete',
    versions: [{ id: 'v1', content }],
  }
}

function createVideoMessage(content: string): Message {
  return {
    key: `assistant-${Date.now()}`,
    from: 'assistant',
    mode: 'video',
    status: 'complete',
    versions: [{ id: 'v1', content }],
  }
}

function getTestConsole(): Pick<Console, 'error'> {
  return Reflect.get(globalThis, 'console') as Pick<Console, 'error'>
}

describe('playground message storage', () => {
  let localStorageMock: LocalStorageMock
  const originalLocalStorage = globalThis.localStorage
  const originalConsoleError = getTestConsole().error

  beforeEach(() => {
    localStorageMock = new LocalStorageMock()
    Object.defineProperty(globalThis, 'localStorage', {
      value: localStorageMock,
      configurable: true,
    })
  })

  afterEach(() => {
    getTestConsole().error = originalConsoleError
    localStorageMock.clear()
    Object.defineProperty(globalThis, 'localStorage', {
      value: originalLocalStorage,
      configurable: true,
    })
  })

  test('preserves generated image data urls when loading from storage', () => {
    const scope = 'image-history'
    const imageMarkdown = `![Generated image 1](data:image/png;base64,${'A'.repeat(60_000)})`

    saveMessages([createImageMessage(imageMarkdown)], scope)
    const loaded = loadMessages(scope)

    assert.equal(loaded?.[0]?.versions[0]?.content, imageMarkdown)
    clearPlaygroundData(scope)
  })

  test('uses memory cache when browser storage rejects image history', () => {
    const scope = 'image-history-memory-fallback'
    const imageMarkdown = `![Generated image 1](data:image/png;base64,${'A'.repeat(60_000)})`
    getTestConsole().error = () => {}
    localStorageMock.throwOnSet = true

    saveMessages([createImageMessage(imageMarkdown)], scope)
    const loaded = loadMessages(scope)

    assert.equal(loaded?.[0]?.versions[0]?.content, imageMarkdown)
    clearPlaygroundData(scope)
  })

  test('preserves generated video mode when loading from storage', () => {
    const scope = 'video-history'

    saveMessages(
      [createVideoMessage('[Video Preview](/v1/videos/task_123/content)')],
      scope
    )
    const loaded = loadMessages(scope)

    assert.equal(loaded?.[0]?.mode, 'video')
    clearPlaygroundData(scope)
  })

  test('preserves user screenshot image urls when loading from storage', () => {
    const scope = 'screenshot-history'
    const message: Message = {
      key: 'user-1',
      from: 'user',
      mode: 'chat',
      versions: [{ id: 'v1', content: 'look at this' }],
      imageUrls: ['data:image/png;base64,abc'],
    }

    saveMessages([message], scope)
    const loaded = loadMessages(scope)

    assert.deepEqual(loaded?.[0]?.imageUrls, ['data:image/png;base64,abc'])
    clearPlaygroundData(scope)
  })

  test('isolates Cursor Agent sessions by playground user', () => {
    const firstSession = {
      agentId: 'bc-00000000-0000-0000-0000-000000000001',
      signature: `v2.${'a'.repeat(64)}`,
      channelId: 10,
      keyIndex: 0,
      model: 'grok-4.5',
      group: 'default',
    }
    const secondSession = {
      agentId: 'bc-00000000-0000-0000-0000-000000000002',
      signature: `v2.${'b'.repeat(64)}`,
      channelId: 20,
      keyIndex: 1,
      model: 'claude-sonnet-4',
      group: 'vip',
    }

    saveCursorAgentSession(firstSession, 1)
    saveCursorAgentSession(secondSession, 2)

    assert.deepEqual(loadCursorAgentSession(1), firstSession)
    assert.deepEqual(loadCursorAgentSession(2), secondSession)
    clearCursorAgentSession(1)
    assert.equal(loadCursorAgentSession(1), null)
    assert.deepEqual(loadCursorAgentSession(2), secondSession)
  })

  test('drops legacy Cursor Agent sessions after a backend restart', () => {
    const scope = 'legacy-cursor-session'
    const storageKey = `playground_cursor_agent_session:user:${scope}`
    getTestConsole().error = () => {}
    localStorageMock.setItem(
      storageKey,
      JSON.stringify({
        version: 1,
        data: {
          agentId: 'bc-00000000-0000-0000-0000-000000000001',
          signature: 'v1.old-instance-signature',
          channelId: 10,
          keyIndex: 0,
        },
      })
    )

    assert.equal(loadCursorAgentSession(scope), null)
    assert.equal(localStorageMock.getItem(storageKey), null)
  })
})
