import type { PlaygroundMode } from '../../types'

function isXAIVideoModel(model: string): boolean {
  return model.startsWith('grok-imagine-video')
}

function isXAIImageModel(model: string): boolean {
  return model.startsWith('grok-imagine-image') || model.startsWith('grok-2-image')
}

export function getPlaygroundTaskModel(
  mode: Exclude<PlaygroundMode, 'chat' | 'speech'>,
  model: string
): string {
  if (mode === 'image' && isXAIVideoModel(model)) {
    return 'grok-imagine-image'
  }

  if (mode === 'video' && isXAIImageModel(model)) {
    return 'grok-imagine-video'
  }

  return model
}
