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

import type { VolcengineQuotaBinding } from '../../types'

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
const { VolcengineUsageCells } = await import('../../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        '5-Hour Window': '5-Hour Window',
        Daily: 'Daily',
        'Weekly Window': 'Weekly Window',
        Monthly: 'Monthly',
        Start: 'Start',
        Reset: 'Reset',
        Used: 'Used',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const binding: VolcengineQuotaBinding = {
  id: 1,
  name: 'Agent Plan',
  note: '',
  enabled: true,
  has_curl: true,
  last_refreshed_at: 0,
  last_error: '',
  created_at: 0,
  updated_at: 0,
  last_plan_type: 'medium',
  last_five_hour_quota: 10_000,
  last_five_hour_used_afp: 11.4375,
  last_five_hour_subscribe_at: 1_785_746_215,
  last_five_hour_reset_at: 1_785_764_215,
  last_daily_quota: 50_000,
  last_daily_used_afp: 500,
  last_daily_subscribe_at: 1_785_686_400,
  last_daily_reset_at: 1_785_772_800,
  last_weekly_quota: 35_000,
  last_weekly_used_afp: 1_969.6252,
  last_weekly_subscribe_at: 1_785_686_400,
  last_weekly_reset_at: 1_786_291_200,
  last_monthly_quota: 100_000,
  last_monthly_used_afp: 16_517.7157,
  last_monthly_subscribe_at: 1_785_745_776,
  last_monthly_reset_at: 1_788_451_199,
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
              <VolcengineUsageCells binding={binding} />
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

describe('VolcEngine AFP usage display', () => {
  test('keeps exact AFP amounts out of the row and exposes them through the tooltip', async () => {
    await renderUsageCells()

    assert.match(rendered?.container.textContent || '', /0\.11%/)
    assert.doesNotMatch(
      document.body.textContent || '',
      /11\.4375 \/ 10,000 AFP/
    )

    const trigger = document.querySelector<HTMLElement>(
      '[data-slot="tooltip-trigger"]'
    )
    assert.ok(trigger)
    assert.equal(trigger.tabIndex, 0)

    await act(async () => trigger.focus())

    assert.match(document.body.textContent || '', /11\.4375 \/ 10,000 AFP/)
  })
})
