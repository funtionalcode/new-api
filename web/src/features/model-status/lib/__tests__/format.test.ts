import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  calculateModelStatusSuccessCount,
  canInspectModelStatusErrors,
  formatModelStatusMs,
  modelStatusVisual,
} from '../format'

describe('model status formatting', () => {
  test('maps every availability status to a stable label key', () => {
    assert.equal(modelStatusVisual('normal').labelKey, 'Normal')
    assert.equal(modelStatusVisual('warning').labelKey, 'Warning')
    assert.equal(modelStatusVisual('error').labelKey, 'Error')
    assert.equal(modelStatusVisual('no_request').labelKey, 'No requests')
  })

  test('allows inspecting only warning and error status details', () => {
    assert.equal(canInspectModelStatusErrors('normal'), false)
    assert.equal(canInspectModelStatusErrors('no_request'), false)
    assert.equal(canInspectModelStatusErrors('warning'), true)
    assert.equal(canInspectModelStatusErrors('error'), true)
  })

  test('shows a dash when latency data is unavailable', () => {
    assert.equal(formatModelStatusMs(0), '-')
    assert.equal(formatModelStatusMs(Number.NaN), '-')
  })

  test('derives a bounded success count for timeline tooltips', () => {
    assert.equal(calculateModelStatusSuccessCount(100, 0.996), 100)
    assert.equal(calculateModelStatusSuccessCount(100, 0.804), 80)
    assert.equal(calculateModelStatusSuccessCount(0, 1), 0)
    assert.equal(calculateModelStatusSuccessCount(10, Number.NaN), 0)
  })
})
