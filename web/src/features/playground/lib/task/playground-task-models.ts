import type { PlaygroundMode } from '../../types'

function isXAIVideoModel(model: string): boolean {
  return model.startsWith('grok-imagine-video')
}

function isXAIImageModel(model: string): boolean {
  return (
    model.startsWith('grok-imagine-image') || model.startsWith('grok-2-image')
  )
}

export function isAudioTranscriptionModel(model: string): boolean {
  const name = model.trim().toLowerCase()

  return (
    name === 'whisper-1' ||
    name === 'grok-stt' ||
    name.includes('transcribe') ||
    name.includes('asr')
  )
}

export function getPlaygroundTaskModel(
  mode: Exclude<PlaygroundMode, 'chat' | 'speech' | 'transcription'>,
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

export function getPlaygroundGenerationMode(
  mode: PlaygroundMode,
  model: string
): PlaygroundMode {
  if (mode !== 'chat') {
    return mode
  }

  if (isXAIImageModel(model)) {
    return 'image'
  }

  if (isXAIVideoModel(model)) {
    return 'video'
  }

  if (isAudioTranscriptionModel(model)) {
    return 'transcription'
  }

  return mode
}
