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

import type { CursorQuotaBinding } from '../../types'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLDivElement',
  'Node',
  'Element',
  'Event',
  'FocusEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
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
const { CursorUsageCells } = await import('../../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'API Usage': 'API Usage',
        'Billing Cycle': 'Billing Cycle',
        'Grok Bot Usage': 'Grok Bot Usage',
        'On-Demand Spend': 'On-Demand Spend',
        'Plan Spend': 'Plan Spend',
        'Reset at:': 'Reset at:',
        'Total Usage': 'Total Usage',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const binding: CursorQuotaBinding = {
  id: 1,
  name: 'Cursor Ultra',
  note: '',
  enabled: true,
  has_curl: true,
  has_usage_amount_curl: true,
  has_usage_cost_curl: true,
  last_refreshed_at: 0,
  last_error: '',
  created_at: 0,
  updated_at: 0,
  last_plan_name: 'Ultra',
  last_billing_cycle_start_at: 1_785_744_000,
  last_billing_cycle_end_at: 1_788_422_400,
  last_plan_used_cents: 5_288,
  last_plan_limit_cents: 40_000,
  last_plan_remaining_cents: 34_712,
  last_plan_api_percent: 10.558,
  last_plan_total_percent: 2.1152,
  last_grok_bot_usage_percent: 37.5,
  last_grok_bot_reset_at: 1_786_838_400,
  last_grok_bot_usage_available: true,
  last_on_demand_used_cents: 2_500,
  last_on_demand_limit_cents: 15_000,
  last_on_demand_remaining_cents: 12_500,
  last_total_input_tokens: 0,
  last_total_output_tokens: 0,
  last_total_cache_write_tokens: 0,
  last_total_cache_read_tokens: 0,
  last_total_cost_cents: 0,
  last_model_usage: '',
}

let rendered:
  | {
      container: HTMLDivElement
      root: ReturnType<typeof createRoot>
    }
  | undefined

async function renderUsageCells() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  rendered = { container, root }

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <table>
          <tbody>
            <tr>
              <CursorUsageCells binding={binding} />
            </tr>
          </tbody>
        </table>
      </I18nextProvider>
    )
  })
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

describe('Cursor Grok Bot usage display', () => {
  test('shows the weekly usage progress and reset alongside Cursor usage', async () => {
    await renderUsageCells()

    const progressBars = rendered?.container.querySelectorAll(
      '[data-slot="progress"]'
    )
    assert.equal(progressBars?.length, 3)

    const grokBotProgress = rendered?.container.querySelector(
      '[aria-label="Grok Bot Usage"]'
    )
    assert.ok(grokBotProgress)
    assert.equal(grokBotProgress.getAttribute('aria-valuenow'), '37.5')
    assert.match(rendered?.container.textContent || '', /Grok Bot Usage37\.5%/)

    const trigger = document.querySelector<HTMLElement>(
      '[data-slot="tooltip-trigger"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.focus())

    assert.match(document.body.textContent || '', /Grok Bot Usage · Reset at:/)
  })
})
