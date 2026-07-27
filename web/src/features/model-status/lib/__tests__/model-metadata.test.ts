import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { ModelStatusModel } from '../../types'
import {
  getModelStatusGroups,
  getModelStatusModelMeta,
} from '../model-metadata'

describe('model status metadata', () => {
  test('maps model families to dedicated LobeHub icons', () => {
    assert.equal(
      getModelStatusModelMeta('claude-opus-4-6').iconKey,
      'Claude.Avatar'
    )
    assert.equal(
      getModelStatusModelMeta('gpt-5.6-sol').iconKey,
      "OpenAI.Avatar.type={'gpt5'}"
    )
    assert.equal(
      getModelStatusModelMeta('o3-mini').iconKey,
      "OpenAI.Avatar.type={'o3'}"
    )
    assert.equal(
      getModelStatusModelMeta('gemini-2.5-pro').iconKey,
      'Gemini.Avatar'
    )
  })

  test('groups models by family in the status page order', () => {
    const models = [
      modelStatusFixture('gemini-2.5-pro'),
      modelStatusFixture('gpt-5.6-sol'),
      modelStatusFixture('claude-sonnet-4-6'),
      modelStatusFixture('custom-router-model'),
      modelStatusFixture('gpt-4o'),
    ]

    const groups = getModelStatusGroups(models)

    assert.deepEqual(
      groups.map((group) => group.key),
      ['claude', 'openai', 'gemini', 'other']
    )
    assert.deepEqual(
      groups
        .find((group) => group.key === 'openai')
        ?.models.map((model) => model.model_name),
      ['gpt-5.6-sol', 'gpt-4o']
    )
  })
})

function modelStatusFixture(modelName: string): ModelStatusModel {
  return {
    model_name: modelName,
    request_count: 0,
    token_count: 0,
    quota: 0,
    success_rate: 0,
    avg_first_response_time_ms: 0,
    avg_latency_ms: 0,
    tps: 0,
    status: 'no_request',
    buckets: [],
  }
}
