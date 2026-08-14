/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

import { API_ENDPOINTS, CURSOR_AGENT_HEADERS } from './constants'
import type {
  AudioTranscriptionRequest,
  AudioTranscriptionResponse,
  ChatCompletionRequest,
  ChatCompletionResponse,
  CursorAgentSession,
  ImageGenerationRequest,
  ImageGenerationResponse,
  ModelOption,
  GroupOption,
  SpeechGenerationRequest,
  VideoGenerationRequest,
  VideoGenerationResponse,
} from './types'

async function attachmentToBlobFile(
  request: AudioTranscriptionRequest
): Promise<{ blob: Blob; filename: string }> {
  if (!request.file.url) {
    throw new Error('Audio file is required')
  }

  const response = await fetch(request.file.url)
  const blob = await response.blob()
  return {
    blob:
      request.file.mediaType && blob.type !== request.file.mediaType
        ? blob.slice(0, blob.size, request.file.mediaType)
        : blob,
    filename: request.file.filename || 'audio',
  }
}

/**
 * Send chat completion request (non-streaming)
 */
export interface ChatCompletionResult {
  data: ChatCompletionResponse
  cursorSession: CursorAgentSession | null
  cursorAgentDeleted: boolean
}

function getCursorSessionFromHeaders(
  headers: Record<string, unknown>,
  payload: ChatCompletionRequest
): CursorAgentSession | null {
  const agentId = String(headers[CURSOR_AGENT_HEADERS.ID.toLowerCase()] ?? '')
  const signature = String(
    headers[CURSOR_AGENT_HEADERS.SIGNATURE.toLowerCase()] ?? ''
  )
  const channelId = Number(
    headers[CURSOR_AGENT_HEADERS.CHANNEL_ID.toLowerCase()]
  )
  const keyIndex = Number(headers[CURSOR_AGENT_HEADERS.KEY_INDEX.toLowerCase()])
  if (
    !agentId ||
    !signature ||
    !Number.isInteger(channelId) ||
    channelId <= 0
  ) {
    return null
  }
  if (!Number.isInteger(keyIndex) || keyIndex < 0) return null
  return {
    agentId,
    signature,
    channelId,
    keyIndex,
    model: payload.model,
    group: payload.group ?? '',
  }
}

export async function sendChatCompletion(
  payload: ChatCompletionRequest,
  requestHeaders: Record<string, string>,
  signal?: AbortSignal
): Promise<ChatCompletionResult> {
  const res = await api.post(API_ENDPOINTS.CHAT_COMPLETIONS, payload, {
    headers: requestHeaders,
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  const headers = res.headers as Record<string, unknown>
  return {
    data: res.data,
    cursorSession: getCursorSessionFromHeaders(headers, payload),
    cursorAgentDeleted:
      String(headers[CURSOR_AGENT_HEADERS.DELETED.toLowerCase()]) === 'true',
  }
}

export function cursorAgentRequestHeaders(
  session: CursorAgentSession | null,
  model: string,
  group?: string
): Record<string, string> {
  const headers: Record<string, string> = {
    [CURSOR_AGENT_HEADERS.PERSISTENT]: 'true',
  }
  if (!session || session.model !== model || session.group !== (group ?? '')) {
    return headers
  }
  return {
    ...headers,
    [CURSOR_AGENT_HEADERS.ID]: session.agentId,
    [CURSOR_AGENT_HEADERS.SIGNATURE]: session.signature,
    [CURSOR_AGENT_HEADERS.CHANNEL_ID]: String(session.channelId),
    [CURSOR_AGENT_HEADERS.KEY_INDEX]: String(session.keyIndex),
  }
}

export async function sendImageGeneration(
  payload: ImageGenerationRequest,
  signal?: AbortSignal
): Promise<ImageGenerationResponse> {
  const res = await api.post(API_ENDPOINTS.IMAGE_GENERATIONS, payload, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function sendSpeechGeneration(
  payload: SpeechGenerationRequest,
  signal?: AbortSignal
): Promise<Blob> {
  const res = await api.post(API_ENDPOINTS.AUDIO_SPEECH, payload, {
    responseType: 'blob',
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function sendAudioTranscription(
  payload: AudioTranscriptionRequest,
  signal?: AbortSignal
): Promise<AudioTranscriptionResponse> {
  const formData = new FormData()
  const { blob, filename } = await attachmentToBlobFile(payload)

  formData.append('model', payload.model)
  if (payload.group) formData.append('group', payload.group)
  if (payload.prompt?.trim()) formData.append('prompt', payload.prompt.trim())
  formData.append('file', blob, filename)

  const res = await api.post(API_ENDPOINTS.AUDIO_TRANSCRIPTIONS, formData, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function sendVideoGeneration(
  payload: VideoGenerationRequest,
  signal?: AbortSignal
): Promise<VideoGenerationResponse> {
  const res = await api.post(API_ENDPOINTS.VIDEO_GENERATIONS, payload, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function getVideoGeneration(
  id: string,
  signal?: AbortSignal
): Promise<VideoGenerationResponse> {
  const res = await api.get(`${API_ENDPOINTS.VIDEO_GENERATIONS}/${id}`, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Get user available models
 */
export async function getUserModels(group: string): Promise<ModelOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_MODELS, {
    params: { group },
  })
  const { data } = res

  if (!data.success || !Array.isArray(data.data)) {
    return []
  }

  return data.data.map((model: string) => ({
    label: model,
    value: model,
  }))
}

/**
 * Get user groups
 */
export async function getUserGroups(): Promise<GroupOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_GROUPS)
  const { data } = res

  if (!data.success || !data.data) {
    return []
  }

  const groupData = data.data as Record<string, { desc: string; ratio: number }>

  // label is for button display (name only); desc is for dropdown content
  return Object.entries(groupData).map(([group, info]) => ({
    label: group,
    value: group,
    ratio: info.ratio,
    desc: info.desc,
  }))
}
