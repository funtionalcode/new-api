import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  formatLogContextSize,
  getResponseTimeColor,
  isWebsocketLog,
} from './format'

describe('usage log timing colors', () => {
  test('colors response time by elapsed seconds instead of throughput', () => {
    assert.equal(getResponseTimeColor(6, 1000), 'success')
    assert.equal(getResponseTimeColor(20, 1000), 'warning')
    assert.equal(getResponseTimeColor(35, 1000), 'danger')
  })
})

describe('usage log websocket marker', () => {
  test('detects explicit ws and websocket transport markers', () => {
    assert.equal(isWebsocketLog({ ws: true }), true)
    assert.equal(isWebsocketLog({ transport: 'websocket' }), true)
    assert.equal(isWebsocketLog({ transport: 'direct' }), false)
    assert.equal(isWebsocketLog(null), false)
  })
})

describe('usage log context size formatting', () => {
  test('uses fixed K, M, and 亿 units for large token counts', () => {
    assert.equal(formatLogContextSize(999), '999')
    assert.equal(formatLogContextSize(1_000), '1K')
    assert.equal(formatLogContextSize(131_328), '131.3K')
    assert.equal(formatLogContextSize(1_250_000), '1.3M')
    assert.equal(formatLogContextSize(125_000_000), '1.3亿')
  })
})
