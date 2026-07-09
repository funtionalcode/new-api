import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { readClipboardImageFiles } from './clipboard-image-utils'

describe('clipboard image utils', () => {
  test('reads image files from clipboard items', async () => {
    const imageBlob = new Blob(['png'], { type: 'image/png' })

    const files = await readClipboardImageFiles(async () => [
      {
        types: ['text/plain', 'image/png'],
        getType: async (type: string) => {
          assert.equal(type, 'image/png')
          return imageBlob
        },
      },
    ])

    assert.equal(files.length, 1)
    assert.equal(files[0].type, 'image/png')
    assert.match(files[0].name, /^screenshot-\d+\.png$/)
  })

  test('ignores non-image clipboard items', async () => {
    const files = await readClipboardImageFiles(async () => [
      {
        types: ['text/plain'],
        getType: async () => new Blob(['text'], { type: 'text/plain' }),
      },
    ])

    assert.deepEqual(files, [])
  })
})
