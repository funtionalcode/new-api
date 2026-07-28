import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  MODEL_STATUS_ERROR_LOG_TYPE,
  MODEL_STATUS_ERROR_TIME_WINDOW_MS,
  buildModelStatusErrorUsageLogSearch,
} from '../error-log-link'

describe('model status error log link', () => {
  test('includes request id and a narrow time window when available', () => {
    const createdAt = 1_753_689_385
    const search = buildModelStatusErrorUsageLogSearch({
      modelName: 'gpt-5.6-sol',
      detail: {
        created_at: createdAt,
        message: 'status_code=500',
        request_id: 'req-abc',
      },
    })

    assert.deepEqual(search, {
      model: 'gpt-5.6-sol',
      type: [MODEL_STATUS_ERROR_LOG_TYPE],
      page: 1,
      requestId: 'req-abc',
      startTime: createdAt * 1000 - MODEL_STATUS_ERROR_TIME_WINDOW_MS,
      endTime: createdAt * 1000 + MODEL_STATUS_ERROR_TIME_WINDOW_MS,
    })
  })

  test('falls back to model, error type and a narrow time window', () => {
    const createdAt = 1_753_689_385
    const search = buildModelStatusErrorUsageLogSearch({
      modelName: ' gpt-5.6-sol ',
      detail: {
        created_at: createdAt,
        message: 'status_code=500',
      },
    })

    assert.equal(search.model, 'gpt-5.6-sol')
    assert.deepEqual(search.type, [MODEL_STATUS_ERROR_LOG_TYPE])
    assert.equal(search.page, 1)
    assert.equal(search.requestId, undefined)
    assert.equal(
      search.startTime,
      createdAt * 1000 - MODEL_STATUS_ERROR_TIME_WINDOW_MS
    )
    assert.equal(
      search.endTime,
      createdAt * 1000 + MODEL_STATUS_ERROR_TIME_WINDOW_MS
    )
  })
})
