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
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
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

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Log Details': 'Log Details',
        Manage: 'Manage',
        'View the complete details for this log entry':
          'View the complete details for this log entry',
        'Operator Admin': 'Operator Admin',
        'Target User': 'Target User',
        'Operation Audit Info': 'Operation Audit Info',
        Operation: 'Operation',
        'Authentication Method': 'Authentication Method',
        Session: 'Session',
        'Increased user quota by {{quota}}':
          'Increased user quota by {{quota}}',
        Content: 'Content',
      },
    },
  },
})

const { DetailsDialog } = await import('../dialogs/details-dialog')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const quotaAuditLog: UsageLog = {
  id: 1,
  user_id: 1,
  created_at: 1,
  type: 3,
  content: 'Increased user quota by $50.000000 额度',
  username: 'root',
  remark: '',
  token_name: '',
  model_name: '',
  quota: 0,
  prompt_tokens: 0,
  completion_tokens: 0,
  use_time: 0,
  is_stream: false,
  channel: 0,
  channel_name: '',
  token_id: 0,
  group: '',
  ip: '192.168.97.1',
  other: JSON.stringify({
    admin_info: {
      admin_username: 'root',
      admin_id: 1,
      auth_method: 'session',
    },
    op: {
      action: 'user.quota_add',
      params: {
        quota: '$50.000000 额度',
        target_user_id: 42,
        target_username: 'quota-recipient',
      },
    },
  }),
  request_id: '',
  upstream_request_id: '',
}

describe('usage log audit target user', () => {
  afterAll(() => {
    domWindow.close()
  })

  test('shows which user received the quota adjustment', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <QueryClientProvider client={queryClient}>
            <DetailsDialog
              log={quotaAuditLog}
              isAdmin
              isRoot
              open
              onOpenChange={() => {}}
            />
          </QueryClientProvider>
        </I18nextProvider>
      )
    })

    const text = document.body.textContent ?? ''
    assert.equal(text.includes('Target User'), true)
    assert.equal(text.includes('quota-recipient (ID: 42)'), true)

    await act(async () => root.unmount())
    container.remove()
  })
})
