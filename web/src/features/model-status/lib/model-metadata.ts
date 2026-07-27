import type { ModelStatusModel } from '../types'

type ModelStatusFamilyKey =
  | 'claude'
  | 'openai'
  | 'gemini'
  | 'deepseek'
  | 'qwen'
  | 'moonshot'
  | 'doubao'
  | 'zhipu'
  | 'meta'
  | 'mistral'
  | 'xai'
  | 'perplexity'
  | 'cohere'
  | 'bedrock'
  | 'image'
  | 'other'

type ModelStatusFamily = {
  key: ModelStatusFamilyKey
  label: string
  iconKey: string
}

export type ModelStatusModelMeta = {
  family: ModelStatusFamily
  iconKey: string
}

export type ModelStatusModelGroup = ModelStatusFamily & {
  models: ModelStatusModel[]
}

const OTHER_MODEL_STATUS_FAMILY: ModelStatusFamily = {
  key: 'other',
  label: 'Other models',
  iconKey: 'LlmApi.Avatar',
}

const MODEL_STATUS_FAMILIES: ModelStatusFamily[] = [
  { key: 'claude', label: 'Claude', iconKey: 'Claude.Avatar' },
  {
    key: 'openai',
    label: 'OpenAI',
    iconKey: "OpenAI.Avatar.type={'platform'}",
  },
  { key: 'gemini', label: 'Gemini', iconKey: 'Gemini.Avatar' },
  { key: 'deepseek', label: 'DeepSeek', iconKey: 'DeepSeek.Avatar' },
  { key: 'qwen', label: 'Qwen', iconKey: 'Qwen.Avatar' },
  { key: 'moonshot', label: 'Moonshot', iconKey: 'Moonshot.Avatar' },
  { key: 'doubao', label: 'Doubao', iconKey: 'Doubao.Avatar' },
  { key: 'zhipu', label: 'Zhipu', iconKey: 'Zhipu.Avatar' },
  { key: 'meta', label: 'Meta', iconKey: 'Meta.Avatar' },
  { key: 'mistral', label: 'Mistral', iconKey: 'Mistral.Avatar' },
  { key: 'xai', label: 'xAI', iconKey: 'XAI.Avatar' },
  { key: 'perplexity', label: 'Perplexity', iconKey: 'Perplexity.Avatar' },
  { key: 'cohere', label: 'Cohere', iconKey: 'Cohere.Avatar' },
  { key: 'bedrock', label: 'Bedrock', iconKey: 'Bedrock.Avatar' },
  { key: 'image', label: 'Image models', iconKey: 'Stability.Avatar' },
  OTHER_MODEL_STATUS_FAMILY,
]

const MODEL_STATUS_FAMILY_BY_KEY = new Map(
  MODEL_STATUS_FAMILIES.map((family) => [family.key, family])
)

export function getModelStatusModelMeta(
  modelName: string
): ModelStatusModelMeta {
  const normalizedName = modelName.trim().toLowerCase()

  if (
    normalizedName.includes('claude') ||
    normalizedName.includes('anthropic')
  ) {
    return buildModelStatusMeta('claude', 'Claude.Avatar')
  }

  const openAIIconKey = getOpenAIModelIconKey(normalizedName)
  if (openAIIconKey) {
    return buildModelStatusMeta('openai', openAIIconKey)
  }

  if (normalizedName.includes('gemini') || normalizedName.includes('gemma')) {
    return buildModelStatusMeta('gemini', 'Gemini.Avatar')
  }

  if (normalizedName.includes('deepseek')) {
    return buildModelStatusMeta('deepseek', 'DeepSeek.Avatar')
  }

  if (normalizedName.includes('qwen') || normalizedName.includes('qwq')) {
    return buildModelStatusMeta('qwen', 'Qwen.Avatar')
  }

  if (normalizedName.includes('kimi')) {
    return buildModelStatusMeta('moonshot', 'Kimi.Avatar')
  }

  if (normalizedName.includes('moonshot')) {
    return buildModelStatusMeta('moonshot', 'Moonshot.Avatar')
  }

  if (normalizedName.includes('doubao') || normalizedName.includes('volc')) {
    return buildModelStatusMeta('doubao', 'Doubao.Avatar')
  }

  if (normalizedName.includes('glm') || normalizedName.includes('zhipu')) {
    return buildModelStatusMeta('zhipu', 'ChatGLM.Avatar')
  }

  if (normalizedName.includes('llama') || normalizedName.includes('meta-')) {
    return buildModelStatusMeta('meta', 'Meta.Avatar')
  }

  if (
    normalizedName.includes('mistral') ||
    normalizedName.includes('mixtral') ||
    normalizedName.includes('codestral')
  ) {
    return buildModelStatusMeta('mistral', 'Mistral.Avatar')
  }

  if (normalizedName.includes('grok') || normalizedName.includes('xai')) {
    return buildModelStatusMeta('xai', 'XAI.Avatar')
  }

  if (
    normalizedName.includes('sonar') ||
    normalizedName.includes('pplx') ||
    normalizedName.includes('perplexity')
  ) {
    return buildModelStatusMeta('perplexity', 'Perplexity.Avatar')
  }

  if (
    normalizedName.includes('cohere') ||
    normalizedName.includes('command-r')
  ) {
    return buildModelStatusMeta('cohere', 'Cohere.Avatar')
  }

  if (normalizedName.includes('bedrock') || normalizedName.includes('titan')) {
    return buildModelStatusMeta('bedrock', 'Bedrock.Avatar')
  }

  const imageIconKey = getImageModelIconKey(normalizedName)
  if (imageIconKey) {
    return buildModelStatusMeta('image', imageIconKey)
  }

  return buildModelStatusMeta('other', 'LlmApi.Avatar')
}

export function getModelStatusGroups(
  models: ModelStatusModel[]
): ModelStatusModelGroup[] {
  const groupsByKey = new Map<ModelStatusFamilyKey, ModelStatusModelGroup>()

  for (const model of models) {
    const meta = getModelStatusModelMeta(model.model_name)
    let group = groupsByKey.get(meta.family.key)

    if (!group) {
      group = { ...meta.family, models: [] }
      groupsByKey.set(meta.family.key, group)
    }

    group.models.push(model)
  }

  return MODEL_STATUS_FAMILIES.flatMap((family) => {
    const group = groupsByKey.get(family.key)
    return group ? [group] : []
  })
}

function buildModelStatusMeta(
  familyKey: ModelStatusFamilyKey,
  iconKey: string
): ModelStatusModelMeta {
  const family = MODEL_STATUS_FAMILY_BY_KEY.get(familyKey)
  if (!family) {
    return {
      family: OTHER_MODEL_STATUS_FAMILY,
      iconKey: 'LlmApi.Avatar',
    }
  }

  return { family, iconKey }
}

function getOpenAIModelIconKey(normalizedName: string): string | null {
  if (normalizedName.includes('gpt-oss')) {
    return "OpenAI.Avatar.type={'oss'}"
  }

  if (normalizedName.includes('gpt-5')) {
    return "OpenAI.Avatar.type={'gpt5'}"
  }

  if (normalizedName.includes('gpt-4')) {
    return "OpenAI.Avatar.type={'gpt4'}"
  }

  if (normalizedName.includes('gpt-3')) {
    return "OpenAI.Avatar.type={'gpt3'}"
  }

  if (/^o1(?:[-_.:]|$)/.test(normalizedName)) {
    return "OpenAI.Avatar.type={'o1'}"
  }

  if (/^o3(?:[-_.:]|$)/.test(normalizedName)) {
    return "OpenAI.Avatar.type={'o3'}"
  }

  if (
    normalizedName.includes('openai') ||
    normalizedName.includes('chatgpt') ||
    normalizedName.includes('whisper') ||
    normalizedName.includes('tts-')
  ) {
    return 'OpenAI.Avatar'
  }

  if (normalizedName.includes('dall') || normalizedName.includes('sora')) {
    return normalizedName.includes('sora') ? 'Sora.Avatar' : 'Dalle.Avatar'
  }

  return null
}

function getImageModelIconKey(normalizedName: string): string | null {
  if (
    normalizedName.includes('midjourney') ||
    normalizedName.startsWith('mj-')
  ) {
    return 'Midjourney.Avatar'
  }

  if (normalizedName.includes('flux')) {
    return 'Flux.Avatar'
  }

  if (normalizedName.includes('stable') || normalizedName.includes('sd3')) {
    return 'Stability.Avatar'
  }

  if (normalizedName.includes('kling')) {
    return 'Kling.Avatar'
  }

  if (normalizedName.includes('runway')) {
    return 'Runway.Avatar'
  }

  return null
}
