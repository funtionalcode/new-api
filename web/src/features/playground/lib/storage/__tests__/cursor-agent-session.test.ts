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
import { afterEach, beforeEach, describe, test } from 'node:test'

import type { CursorAgentSession } from '../../../types'
import {
  clearCursorAgentSession,
  loadCursorAgentSession,
  saveCursorAgentSession,
} from '../storage'

class LocalStorageMock {
  private store = new Map<string, string>()

  getItem(key: string): string | null {
    return this.store.get(key) ?? null
  }

  setItem(key: string, value: string): void {
    this.store.set(key, value)
  }

  removeItem(key: string): void {
    this.store.delete(key)
  }

  clear(): void {
    this.store.clear()
  }
}

describe('Cursor Agent session storage', () => {
  let localStorageMock: LocalStorageMock
  const originalLocalStorage = globalThis.localStorage
  const testConsole = Reflect.get(globalThis, 'console') as Pick<
    Console,
    'error'
  >
  const originalConsoleError = testConsole.error

  beforeEach(() => {
    localStorageMock = new LocalStorageMock()
    Object.defineProperty(globalThis, 'localStorage', {
      value: localStorageMock,
      configurable: true,
    })
    testConsole.error = () => undefined
  })

  afterEach(() => {
    testConsole.error = originalConsoleError
    localStorageMock.clear()
    Object.defineProperty(globalThis, 'localStorage', {
      value: originalLocalStorage,
      configurable: true,
    })
  })

  test('preserves the model and group that own the saved channel', () => {
    const session: CursorAgentSession = {
      agentId: 'bc-00000000-0000-0000-0000-000000000001',
      signature: `v2.${'a'.repeat(64)}`,
      channelId: 17,
      keyIndex: 0,
      model: 'grok-4.5',
      group: 'vip',
    }

    saveCursorAgentSession(session, 1)

    assert.deepEqual(loadCursorAgentSession(1), session)
    clearCursorAgentSession(1)
  })

  test('drops sessions saved before model and group scoping was added', () => {
    const scope = 'unscoped-cursor-session'
    const storageKey = `playground_cursor_agent_session:user:${scope}`
    localStorageMock.setItem(
      storageKey,
      JSON.stringify({
        version: 1,
        data: {
          agentId: 'bc-00000000-0000-0000-0000-000000000001',
          signature: `v2.${'a'.repeat(64)}`,
          channelId: 17,
          keyIndex: 0,
        },
      })
    )

    assert.equal(loadCursorAgentSession(scope), null)
    assert.equal(localStorageMock.getItem(storageKey), null)
  })
})
