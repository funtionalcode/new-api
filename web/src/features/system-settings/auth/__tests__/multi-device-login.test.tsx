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
  'HTMLButtonElement',
  'HTMLLabelElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'MutationObserver',
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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { BasicAuthSection } = await import('../basic-auth-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Multi-device Login': 'Multi-device Login',
        'Allow the same account to stay signed in on multiple devices. Login issuance rate limits still apply.':
          'Allow the same account to stay signed in on multiple devices. Login issuance rate limits still apply.',
      },
    },
  },
})

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

async function renderBasicAuthSection() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  rendered = { container, root }

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <BasicAuthSection
            defaultValues={{
              PasswordLoginEnabled: true,
              MultiDeviceLoginEnabled: false,
              PasswordRegisterEnabled: true,
              EmailVerificationEnabled: false,
              RegisterEnabled: true,
              EmailDomainRestrictionEnabled: false,
              EmailAliasRestrictionEnabled: false,
              EmailDomainWhitelist: '',
            }}
          />
        </I18nextProvider>
      </QueryClientProvider>
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

describe('multi-device login setting', () => {
  test('exposes an accessible switch that can be enabled', async () => {
    await renderBasicAuthSection()

    const label = [
      ...document.querySelectorAll<HTMLLabelElement>('label'),
    ].find(
      (candidate) => candidate.textContent?.trim() === 'Multi-device Login'
    )
    assert.ok(label)
    assert.ok(label.htmlFor)
    assert.ok(label.control)
    const switchControl = label
      .closest('[data-slot="form-item"]')
      ?.querySelector<HTMLElement>('[role="switch"]')
    assert.ok(switchControl)
    assert.equal(switchControl.getAttribute('role'), 'switch')
    const labelledBy = switchControl.getAttribute('aria-labelledby')
    assert.ok(labelledBy)
    assert.equal(
      document.querySelector(`#${labelledBy}`)?.textContent?.trim(),
      'Multi-device Login'
    )
    assert.equal(switchControl.getAttribute('aria-checked'), 'false')

    await act(async () => {
      switchControl.click()
    })

    assert.equal(switchControl.getAttribute('aria-checked'), 'true')
  })
})
