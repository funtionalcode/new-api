import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getPlaygroundTaskModel } from './playground-task-models'

describe('playground task model routing', () => {
  test('uses an xAI image model when retrying image mode from a video model selection', () => {
    assert.equal(
      getPlaygroundTaskModel('image', 'grok-imagine-video'),
      'grok-imagine-image'
    )
  })

  test('uses an xAI video model when retrying video mode from an image model selection', () => {
    assert.equal(
      getPlaygroundTaskModel('video', 'grok-imagine-image-quality'),
      'grok-imagine-video'
    )
  })
})
