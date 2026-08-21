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

import { Window } from 'happy-dom'
import { afterAll, afterEach, describe, test } from 'vitest'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLTextAreaElement',
  'Node',
  'Element',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { ChannelKeyDisplay } = await import('../channel-key-display')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let rendered:
  | {
      container: HTMLDivElement
      root: ReturnType<typeof createRoot>
    }
  | undefined

async function renderChannelKeyDisplay(value: string | null) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  rendered = { container, root }

  await act(async () => {
    root.render(
      <ChannelKeyDisplay
        label='Current key'
        placeholder='Hidden key'
        value={value}
      />
    )
  })

  const field = container.querySelector<HTMLTextAreaElement>(
    'textarea[aria-label="Current key"]'
  )
  assert.ok(field)
  return field
}

afterEach(async () => {
  if (!rendered) return
  await act(async () => rendered?.root.unmount())
  rendered.container.remove()
  rendered = undefined
})

afterAll(() => {
  domWindow.close()
})

describe('channel key display', () => {
  test('shows every configured key on its own line', async () => {
    const keys = ['key-one', 'key-two', 'key-three'].join('\n')

    const field = await renderChannelKeyDisplay(keys)

    assert.equal(field.value, keys)
    assert.equal(field.getAttribute('rows'), '3')
    assert.equal(field.readOnly, true)
  })

  test('keeps all keys available when the visible row count is capped', async () => {
    const keys = Array.from(
      { length: 12 },
      (_, index) => `key-${index + 1}`
    ).join('\n')

    const field = await renderChannelKeyDisplay(keys)

    assert.equal(field.value, keys)
    assert.equal(field.getAttribute('rows'), '8')
  })

  test('shows the hidden placeholder before verification', async () => {
    const field = await renderChannelKeyDisplay(null)

    assert.equal(field.value, '')
    assert.equal(field.placeholder, 'Hidden key')
    assert.equal(field.getAttribute('rows'), '1')
  })
})
