import assert from 'node:assert/strict'

import { describe, expect, test } from 'vitest'

import type { ModelStatusModel } from '../../types'
import {
  getModelStatusGroups,
  getModelStatusModelMeta,
} from '../model-metadata'

describe('model status metadata', () => {
  test.each([
    'gpt-6-astra',
    'gpt-6.1-sol',
    'gpt-6-luna',
    'gpt-6-sol',
    'gpt-image-2.5-sunburst',
    'openai/gpt-7-preview',
    'o4-mini',
  ])('缺少供应商时将 %s 归入 OpenAI', (name) => {
    expect(getModelStatusModelMeta(name).family.key).toBe('openai')
  })

  test('供应商配置优先于模型名称猜测，保留模型图标', () => {
    const model = modelStatusFixture('gpt-6-astra')
    Object.assign(model, {
      vendor_name: 'Anthropic',
      vendor_icon: 'Claude.Color',
      icon: 'Claude.Avatar',
    })
    const meta = getModelStatusModelMeta(model.model_name, model)
    expect(meta.family.key).toBe('claude')
    expect(meta.iconKey).toBe('Claude.Avatar')
  })

  test('保留自定义供应商分组，并为 Jev 提供 TypeSafe 降级分组', () => {
    const custom = modelStatusFixture('custom-model')
    Object.assign(custom, {
      vendor_name: 'Custom Labs',
      vendor_icon: 'DeepSeek.Color',
    })
    const groups = getModelStatusGroups([
      custom,
      modelStatusFixture('jev-latest'),
      modelStatusFixture('unknown-model'),
    ])
    expect(groups.map((group) => group.label)).toEqual([
      'TypeSafe',
      'Custom Labs',
      'Other models',
    ])
    expect(groups[1].iconKey).toBe('DeepSeek.Color')
    expect(getModelStatusModelMeta('jevish').family.key).toBe('other')
  })

  test('GPT 图片按供应商归类，其他供应商的图片视频模型保持原分组', () => {
    expect(getModelStatusModelMeta('gpt-image-2').family.key).toBe('openai')
    expect(getModelStatusModelMeta('grok-imagine-video-1.5').family.key).toBe(
      'xai'
    )
    expect(getModelStatusModelMeta('qwen-image-2.1').family.key).toBe('qwen')
    expect(getModelStatusGroups([])).toEqual([])
  })

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
