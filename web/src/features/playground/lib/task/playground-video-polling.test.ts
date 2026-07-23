import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { pollVideoGeneration } from './playground-video-polling'
import type { VideoGenerationResponse } from '../../types'

describe('playground video polling', () => {
  test('keeps polling until video completes after the previous 90 attempt limit', async () => {
    const signal = new AbortController().signal
    let fetchCount = 0

    const result = await pollVideoGeneration({
      videoId: 'video_1',
      initialVideo: createVideo('video_1', 'queued'),
      signal,
      failureMessage: 'Video generation failed',
      waitForPollInterval: async () => {},
      getVideoGeneration: async () => {
        fetchCount += 1

        if (fetchCount <= 90) {
          return createVideo('video_1', 'in_progress')
        }

        return {
          ...createVideo('video_1', 'completed'),
          metadata: { url: 'https://example.com/video.mp4' },
        }
      },
    })

    assert.equal(fetchCount, 91)
    assert.equal(result.status, 'completed')
    assert.equal(result.metadata?.url, 'https://example.com/video.mp4')
  })

  test('throws upstream failure messages while polling', async () => {
    const signal = new AbortController().signal

    await assert.rejects(
      pollVideoGeneration({
        videoId: 'video_1',
        initialVideo: createVideo('video_1', 'queued'),
        signal,
        failureMessage: 'Video generation failed',
        waitForPollInterval: async () => {},
        getVideoGeneration: async () => ({
          ...createVideo('video_1', 'failed'),
          error: { message: 'blocked by upstream' },
        }),
      }),
      /blocked by upstream/
    )
  })
})

function createVideo(id: string, status: string): VideoGenerationResponse {
  return {
    id,
    status,
  }
}
