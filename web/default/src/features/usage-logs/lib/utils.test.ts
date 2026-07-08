import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildStatsApiParams } from './utils'

describe('usage log API params', () => {
  test('uses the selected time range as the average use time window', () => {
    const startTime = new Date('2026-07-08T00:00:00+08:00').getTime()
    const endTime = new Date('2026-07-08T10:57:00+08:00').getTime()

    const params = buildStatsApiParams({
      searchParams: {
        startTime,
        endTime,
        type: ['5'],
        model: 'gpt-test',
        token: 'tok',
        channel: '9',
        channelName: 'codex',
        group: 'default',
        username: 'root',
        ip: '192.0.2.1',
        requestId: 'req-test',
        upstreamRequestId: 'up-test',
      },
      isAdmin: true,
    })

    assert.equal(params.start_timestamp, Math.floor(startTime / 1000))
    assert.equal(params.end_timestamp, Math.floor(endTime / 1000))
    assert.equal(params.avg_start_timestamp, params.start_timestamp)
    assert.equal(params.avg_end_timestamp, params.end_timestamp)
    assert.equal(params.type, 5)
    assert.equal(params.model_name, 'gpt-test')
    assert.equal(params.token_name, 'tok')
    assert.equal(params.channel, 9)
    assert.equal(params.channel_name, 'codex')
    assert.equal(params.group, 'default')
    assert.equal(params.username, 'root')
    assert.equal(params.ip, '192.0.2.1')
    assert.equal(params.request_id, 'req-test')
    assert.equal(params.upstream_request_id, 'up-test')
  })
})
