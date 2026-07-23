import assert from 'node:assert/strict'
import { afterEach, describe, test } from 'node:test'

import { DEFAULT_SYSTEM_NAME, DEFAULT_LOGO } from '@/lib/constants'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { formatQuota, formatTokenDetails, formatTokens } from './format'

afterEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      systemName: DEFAULT_SYSTEM_NAME,
      logo: DEFAULT_LOGO,
      currency: { ...DEFAULT_CURRENCY_CONFIG },
    },
  }))
})

describe('token formatting', () => {
  test('uses Chinese token units for dashboard-sized values', () => {
    assert.equal(formatTokens(0), '0')
    assert.equal(formatTokens(10), '10')
    assert.equal(formatTokens(100), '百')
    assert.equal(formatTokens(1000), '千')
    assert.equal(formatTokens(10_000), '万')
    assert.equal(formatTokens(100_000), '10万')
    assert.equal(formatTokens(1_000_000), '百万')
    assert.equal(formatTokens(10_000_000), '千万')
    assert.equal(formatTokens(100_000_000), '亿')
    assert.equal(formatTokens(1_000_000_000), '10亿')
    assert.equal(formatTokens(10_000_000_000), '100亿')
  })

  test('keeps exact token details for tooltips', () => {
    assert.equal(formatTokenDetails(123_456_789), '123,456,789 token')
  })

  test('uses Chinese token units when quota display mode is tokens', () => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'TOKENS',
        },
      },
    }))

    assert.equal(formatQuota(413_000_000), '4.1亿')
  })
})
