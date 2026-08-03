import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getPromptInputAudioAttachments,
  getPromptInputAttachments,
  getPromptInputImageUrls,
  isPromptInputAudioAttachment,
} from './input-control-utils'

describe('playground input attachment utilities', () => {
  test('normalizes uploaded attachments without dropping non-image files', () => {
    assert.deepEqual(
      getPromptInputAttachments({
        files: [
          {
            url: ' data:audio/mpeg;base64,abc ',
            mediaType: ' audio/mpeg ',
            filename: ' speech.mp3 ',
          },
        ],
      }),
      [
        {
          url: 'data:audio/mpeg;base64,abc',
          mediaType: 'audio/mpeg',
          filename: 'speech.mp3',
        },
      ]
    )
  })

  test('detects browser audio files by MIME type or filename', () => {
    assert.equal(
      isPromptInputAudioAttachment({
        mediaType: 'audio/wav',
        filename: 'recording',
      }),
      true
    )
    assert.equal(
      isPromptInputAudioAttachment({
        mediaType: '',
        filename: 'meeting.MP3',
      }),
      true
    )
    assert.equal(
      isPromptInputAudioAttachment({
        mediaType: 'image/png',
        filename: 'image.png',
      }),
      false
    )
  })

  test('separates image URLs from audio attachments', () => {
    const message = {
      files: [
        {
          url: 'data:image/png;base64,abc',
          mediaType: 'image/png',
          filename: 'image.png',
        },
        {
          url: 'data:audio/mpeg;base64,def',
          mediaType: 'audio/mpeg',
          filename: 'audio.mp3',
        },
      ],
    }

    assert.deepEqual(getPromptInputImageUrls(message), [
      'data:image/png;base64,abc',
    ])
    assert.deepEqual(getPromptInputAudioAttachments(message), [
      {
        url: 'data:audio/mpeg;base64,def',
        mediaType: 'audio/mpeg',
        filename: 'audio.mp3',
      },
    ])
  })
})
