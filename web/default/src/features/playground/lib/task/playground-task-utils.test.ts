import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildImageGenerationMarkdown,
  buildSpeechGenerationMarkdown,
} from './playground-task-utils'

describe('playground task utils', () => {
  test('builds markdown for generated image urls', () => {
    const markdown = buildImageGenerationMarkdown({
      data: [{ url: 'https://example.com/image.png' }],
    })

    assert.equal(markdown, '![Generated image 1](https://example.com/image.png)')
  })

  test('builds markdown for generated speech audio', () => {
    const markdown = buildSpeechGenerationMarkdown('blob:http://localhost/audio')

    assert.equal(markdown, '[Audio Preview](blob:http://localhost/audio)')
  })
})
