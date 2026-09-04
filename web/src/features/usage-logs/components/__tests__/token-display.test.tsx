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
import { afterAll, describe, test } from 'vitest'

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
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
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
  resources: { en: { translation: { Cache: 'Cache' } } },
})

const { UsageLogTokens } = await import('../usage-log-tokens')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function createUsageLog(
  promptTokens: number,
  completionTokens: number,
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
    model_name: 'claude-sonnet-5',
    quota: 0,
    prompt_tokens: promptTokens,
    completion_tokens: completionTokens,
    use_time: 0,
    is_stream: true,
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

async function renderTokens(log: UsageLog): Promise<{
  container: HTMLDivElement
  unmount: () => Promise<void>
}> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <UsageLogTokens log={log} />
      </I18nextProvider>
    )
  })

  return {
    container,
    unmount: async () => {
      await act(async () => root.unmount())
      container.remove()
    },
  }
}

describe('usage log token display', () => {
  afterAll(() => {
    domWindow.close()
  })

  test('shows Anthropic cache reads and writes in the total input count', async () => {
    const view = await renderTokens(
      createUsageLog(2, 1_207, {
        usage_semantic: 'anthropic',
        cache_tokens: 81_542,
        cache_creation_tokens: 16_624,
      })
    )

    assert.match(view.container.textContent ?? '', /98,168 \/ 1,207/)
    assert.match(view.container.textContent ?? '', /Cache↓ 81,542/)
    assert.match(view.container.textContent ?? '', /↑ 16,624/)

    await view.unmount()
  })

  test('keeps OpenAI prompt tokens unchanged when cache details are present', async () => {
    const view = await renderTokens(
      createUsageLog(403_151, 130, {
        usage_semantic: 'openai',
        cache_tokens: 401_024,
      })
    )

    assert.match(view.container.textContent ?? '', /403,151 \/ 130/)
    assert.doesNotMatch(view.container.textContent ?? '', /804,175/)

    await view.unmount()
  })
})
