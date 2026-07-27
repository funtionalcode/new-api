import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { formatModelStatusMs, modelStatusVisual } from '../format'

describe('model status formatting', () => {
  test('maps every availability status to a stable label key', () => {
    assert.equal(modelStatusVisual('normal').labelKey, 'Normal')
    assert.equal(modelStatusVisual('warning').labelKey, 'Warning')
    assert.equal(modelStatusVisual('error').labelKey, 'Error')
    assert.equal(modelStatusVisual('no_request').labelKey, 'No requests')
  })

  test('shows a dash when latency data is unavailable', () => {
    assert.equal(formatModelStatusMs(0), '-')
    assert.equal(formatModelStatusMs(Number.NaN), '-')
  })
})
