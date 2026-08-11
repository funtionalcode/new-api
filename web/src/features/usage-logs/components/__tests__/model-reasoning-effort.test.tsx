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
        'Reasoning Effort': 'Reasoning Effort',
      },
    },
  },
})

const { UsageLogModelCell } = await import('../usage-log-model-cell')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function createUsageLog(other: Record<string, unknown>): UsageLog {
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
    prompt_tokens: 0,
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

describe('usage log model reasoning effort', () => {
  after(() => {
    domWindow.close()
  })

  test('shows the reasoning effort as an accessible icon next to the model', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <UsageLogModelCell
            log={createUsageLog({ reasoning_effort: 'high' })}
          />
        </I18nextProvider>
      )
    })

    const reasoningIndicator = container.querySelector(
      '[data-reasoning-effort="high"]'
    )

    assert.equal(container.textContent?.includes('gpt-5.4'), true)
    assert.ok(reasoningIndicator)
    assert.equal(
      reasoningIndicator.getAttribute('title'),
      'Reasoning Effort: high'
    )
    assert.equal(
      reasoningIndicator.getAttribute('aria-label'),
      'Reasoning Effort: high'
    )
    assert.ok(reasoningIndicator.querySelector('svg'))
    assert.equal(
      container.textContent?.includes('Reasoning Effort: high'),
      false
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('does not show an empty reasoning badge for legacy logs', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <UsageLogModelCell log={createUsageLog({})} />
        </I18nextProvider>
      )
    })

    assert.equal(container.textContent?.includes('gpt-5.4'), true)
    assert.equal(container.querySelector('[data-reasoning-effort]'), null)

    await act(async () => root.unmount())
    container.remove()
  })
})
