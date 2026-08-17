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
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { UsageLog } from '../../data/schema'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
Object.defineProperty(domWindow.document, 'compatMode', {
  configurable: true,
  value: 'CSS1Compat',
})
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'PointerEvent',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'customElements',
  'getComputedStyle',
  'matchMedia',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Context Size': 'Context Size',
      },
    },
  },
})

const { UsageLogModelCell } = await import('../usage-log-model-cell')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function createUsageLog(
  promptTokens: number,
  other: Record<string, unknown>
): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 2,
    content: '',
    username: 'demo',
    remark: '',
    token_name: '',
    model_name: 'gpt-5.4',
    quota: 0,
    prompt_tokens: promptTokens,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 0,
    channel_name: '',
    token_id: 0,
    group: '',
    ip: '',
    other: JSON.stringify(other),
    request_id: '',
    upstream_request_id: '',
  }
}

describe('usage log model context size', () => {
  after(() => {
    domWindow.close()
  })

  test('shows the normalized input context in an accessible tooltip', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <UsageLogModelCell
            log={createUsageLog(100_000, { input_tokens_total: 131_328 })}
          />
        </I18nextProvider>
      )
    })

    const contextIndicator = container.querySelector(
      '[data-context-size="131328"]'
    )
    assert.ok(contextIndicator)
    assert.equal(
      contextIndicator.getAttribute('aria-label'),
      'Context Size: 131.3K'
    )
    assert.ok(contextIndicator.querySelector('svg'))

    await act(async () => {
      ;(contextIndicator as HTMLElement).focus()
      await Promise.resolve()
    })

    assert.equal(
      document.body.textContent?.includes('Context Size: 131.3K'),
      true
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('adds Anthropic cache reads and writes to legacy input tokens', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <UsageLogModelCell
            log={createUsageLog(1_000, {
              usage_semantic: 'anthropic',
              cache_tokens: 500,
              cache_write_tokens: 200,
            })}
          />
        </I18nextProvider>
      )
    })

    const contextIndicator = container.querySelector(
      '[data-context-size="1700"]'
    )
    assert.ok(contextIndicator)
    assert.equal(
      contextIndicator.getAttribute('aria-label'),
      'Context Size: 1.7K'
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('does not render a context icon when no input usage is available', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <UsageLogModelCell log={createUsageLog(0, {})} />
        </I18nextProvider>
      )
    })

    assert.equal(container.querySelector('[data-context-size]'), null)

    await act(async () => root.unmount())
    container.remove()
  })
})
