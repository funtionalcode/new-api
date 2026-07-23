import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getPlaygroundGenerationMode,
  getPlaygroundTaskModel,
} from './playground-task-models'

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

  test('routes xAI image-only models to the image endpoint even from chat retries', () => {
    assert.equal(
      getPlaygroundGenerationMode('chat', 'grok-imagine-image-quality'),
      'image'
    )
  })

  test('routes xAI video-only models to the video endpoint even from chat retries', () => {
    assert.equal(
      getPlaygroundGenerationMode('chat', 'grok-imagine-video-1.5'),
      'video'
    )
  })
})
