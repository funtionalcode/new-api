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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { QuotaBindingFormState } from '../../lib/form-payload'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLTextAreaElement',
  'Node',
  'Element',
  'Event',
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

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { DeepSeekCurlFields } = await import('../deepseek-curl-fields')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Account Summary Curl': 'Account Summary Curl',
        'Usage Amount Curl': 'Usage Amount Curl',
        'Usage Cost Curl': 'Usage Cost Curl',
        'Leave blank to keep unchanged': 'Leave blank to keep unchanged',
        'Paste the DeepSeek account summary curl command':
          'Paste the DeepSeek account summary curl command',
        'Paste the DeepSeek usage amount curl command':
          'Paste the DeepSeek usage amount curl command',
        'Paste the DeepSeek usage cost curl command':
          'Paste the DeepSeek usage cost curl command',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const createForm: QuotaBindingFormState = {
  name: 'DeepSeek account',
  note: '',
  request_curl: '',
  usage_amount_curl: '',
  usage_cost_curl: '',
  refresh_token: '',
  proxy: '',
  enabled: true,
  plan_type: '',
  five_hour_limit_tokens: 0,
  weekly_limit_tokens: 0,
}

let rendered:
  | {
      container: HTMLDivElement
      root: ReturnType<typeof createRoot>
    }
  | undefined

function Harness(props: { initialForm: QuotaBindingFormState }) {
  const [form, setForm] = useState(props.initialForm)

  return (
    <I18nextProvider i18n={i18n}>
      <DeepSeekCurlFields
        form={form}
        onChange={(patch) => setForm((current) => ({ ...current, ...patch }))}
      />
    </I18nextProvider>
  )
}

async function renderFields(form: QuotaBindingFormState) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  rendered = { container, root }

  await act(async () => root.render(<Harness initialForm={form} />))
}

function getTextareaByLabel(labelText: string): HTMLTextAreaElement {
  const label = [...document.querySelectorAll<HTMLLabelElement>('label')].find(
    (candidate) => candidate.textContent?.trim() === labelText
  )
  assert.ok(label, `Expected label "${labelText}"`)
  assert.ok(label.htmlFor)
  assert.ok(label.control instanceof domWindow.HTMLTextAreaElement)
  return label.control as unknown as HTMLTextAreaElement
}

async function changeTextarea(textarea: HTMLTextAreaElement, value: string) {
  await act(async () => {
    const valueSetter = Object.getOwnPropertyDescriptor(
      domWindow.HTMLTextAreaElement.prototype,
      'value'
    )?.set
    assert.ok(valueSetter)
    valueSetter.call(textarea, value)
    textarea.dispatchEvent(
      new domWindow.Event('input', { bubbles: true }) as unknown as Event
    )
  })
}

afterEach(async () => {
  if (!rendered) return
  await act(async () => rendered?.root.unmount())
  rendered.container.remove()
  rendered = undefined
})

after(() => {
  domWindow.close()
})

describe('DeepSeek curl fields', () => {
  test('supports entering the three endpoint curl commands independently', async () => {
    await renderFields(createForm)

    const summaryCurl = getTextareaByLabel('Account Summary Curl')
    const amountCurl = getTextareaByLabel('Usage Amount Curl')
    const costCurl = getTextareaByLabel('Usage Cost Curl')
    assert.equal(document.querySelectorAll('textarea').length, 3)
    assert.equal(
      summaryCurl.placeholder,
      'Paste the DeepSeek account summary curl command'
    )
    assert.equal(
      amountCurl.placeholder,
      'Paste the DeepSeek usage amount curl command'
    )
    assert.equal(
      costCurl.placeholder,
      'Paste the DeepSeek usage cost curl command'
    )

    await changeTextarea(summaryCurl, 'curl summary')
    await changeTextarea(amountCurl, 'curl amount')
    await changeTextarea(costCurl, 'curl cost')

    assert.equal(summaryCurl.value, 'curl summary')
    assert.equal(amountCurl.value, 'curl amount')
    assert.equal(costCurl.value, 'curl cost')
  })
})
