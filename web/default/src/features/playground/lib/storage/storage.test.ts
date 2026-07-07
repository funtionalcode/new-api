import assert from 'node:assert/strict'
import { afterEach, beforeEach, describe, test } from 'node:test'

import { clearPlaygroundData, loadMessages, saveMessages } from './storage'
import type { Message } from '../../types'

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
})
