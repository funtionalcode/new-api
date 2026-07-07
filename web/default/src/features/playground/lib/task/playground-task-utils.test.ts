import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildImageGenerationMarkdown,
  buildSpeechGenerationMarkdown,
  extractGeneratedImageUrls,
  extractGeneratedSpeechUrl,
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

  test('extracts generated image urls for custom rendering', () => {
    const markdown = buildImageGenerationMarkdown({
      data: [{ b64_json: 'abc123' }],
    })

    assert.deepEqual(extractGeneratedImageUrls(markdown), [
      'data:image/png;base64,abc123',
    ])
  })

  test('normalizes raw base64 image urls before rendering', () => {
    const markdown = buildImageGenerationMarkdown({
      data: [{ url: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB' }],
    })

    assert.equal(
      markdown,
      '![Generated image 1](data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB)'
    )
  })

  test('extracts raw base64 image content only when allowed', () => {
    const rawImage = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB'

    assert.deepEqual(extractGeneratedImageUrls(rawImage), [])
    assert.deepEqual(
      extractGeneratedImageUrls(rawImage, { allowRawBase64: true }),
      [`data:image/png;base64,${rawImage}`]
    )
  })

  test('extracts generated speech audio url for custom rendering', () => {
    assert.equal(
      extractGeneratedSpeechUrl('[Audio Preview](blob:http://localhost/audio)'),
      'blob:http://localhost/audio'
    )
  })

  test('extracts raw base64 speech content only when allowed', () => {
    const rawAudio = 'SUQzBAAAAAAAI1RTU0UAAAAPAAADTGF2ZjU4'

    assert.equal(extractGeneratedSpeechUrl(rawAudio), null)
    assert.equal(
      extractGeneratedSpeechUrl(rawAudio, { allowRawBase64: true }),
      `data:audio/mpeg;base64,${rawAudio}`
    )
  })
})
