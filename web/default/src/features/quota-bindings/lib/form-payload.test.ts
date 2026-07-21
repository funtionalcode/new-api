import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildQuotaBindingSavePayload } from './form-payload'

describe('quota binding form payload', () => {
  test('omits untouched sensitive fields when editing an existing binding', () => {
    const payload = buildQuotaBindingSavePayload({
      id: 12,
      name: 'glm1',
      note: 'note',
      request_curl: '',
      refresh_token: '',
      proxy: '',
      enabled: true,
      plan_type: 'standard',
      five_hour_limit_tokens: 60_000_000,
      weekly_limit_tokens: 300_000_000,
    })

    assert.equal('request_curl' in payload, false)
    assert.equal('refresh_token' in payload, false)
    assert.equal('proxy' in payload, false)
  })

  test('keeps explicit proxy clearing when the field was changed', () => {
    const payload = buildQuotaBindingSavePayload({
      id: 12,
      name: 'glm1',
      note: '',
      request_curl: '',
      refresh_token: '',
      proxy: '',
      proxy_touched: true,
      enabled: true,
      plan_type: 'standard',
      five_hour_limit_tokens: 60_000_000,
      weekly_limit_tokens: 300_000_000,
    })

    assert.equal(payload.proxy, '')
    assert.equal('request_curl' in payload, false)
    assert.equal('refresh_token' in payload, false)
  })

  test('includes curl and proxy when creating a binding', () => {
    const payload = buildQuotaBindingSavePayload({
      name: 'deepseek1',
      note: '',
      request_curl: '  curl https://platform.deepseek.com  ',
      refresh_token: '',
      proxy: '  http://127.0.0.1:7990  ',
      enabled: true,
      plan_type: '',
      five_hour_limit_tokens: 0,
      weekly_limit_tokens: 0,
    })

    assert.equal(payload.request_curl, 'curl https://platform.deepseek.com')
    assert.equal(payload.refresh_token, '')
    assert.equal(payload.proxy, 'http://127.0.0.1:7990')
  })

  test('includes touched refresh token when editing a binding', () => {
    const payload = buildQuotaBindingSavePayload({
      id: 8,
      name: 'kimi1',
      note: '',
      request_curl: '',
      refresh_token: '  refresh-token-value  ',
      refresh_token_touched: true,
      proxy: '',
      enabled: true,
      plan_type: '',
      five_hour_limit_tokens: 0,
      weekly_limit_tokens: 0,
    })

    assert.equal(payload.refresh_token, 'refresh-token-value')
    assert.equal('request_curl' in payload, false)
  })
})
