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
import type { ImageGenerationResponse } from '../../types'

type ExtractGeneratedMediaOptions = {
  allowRawBase64?: boolean
}

const base64PayloadPattern = /^[A-Za-z0-9+/_-]+={0,2}$/
const generatedImageMarkdownPattern = /!\[[^\]]*]\(([^)\s]+)\)/g
const speechMarkdownPattern = /\[[^\]]*]\(([^)]+)\)/

function getCleanBase64Payload(value: string): string {
  return value.trim().replaceAll(/\s+/g, '')
}

function isLikelyBase64Payload(value: string): boolean {
  const payload = getCleanBase64Payload(value)

  return (
    payload.length >= 16 &&
    payload.length % 4 !== 1 &&
    base64PayloadPattern.test(payload)
  )
}

function getImageMimeType(payload: string): string {
  if (payload.startsWith('/9j/')) return 'image/jpeg'
  if (payload.startsWith('R0lGOD')) return 'image/gif'
  if (payload.startsWith('UklGR')) return 'image/webp'

  return 'image/png'
}

function getAudioMimeType(payload: string): string {
  if (payload.startsWith('UklGR')) return 'audio/wav'
  if (payload.startsWith('T2dnUw')) return 'audio/ogg'
  if (payload.startsWith('AAAA')) return 'audio/mp4'

  return 'audio/mpeg'
}

function normalizeGeneratedImageUrl(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('data:image/')) return trimmed

  if (isLikelyBase64Payload(trimmed)) {
    const payload = getCleanBase64Payload(trimmed)
    return `data:${getImageMimeType(payload)};base64,${payload}`
  }

  return trimmed
}

function normalizeGeneratedSpeechUrl(
  value: string,
  options: ExtractGeneratedMediaOptions = {}
): string | null {
  const trimmed = value.trim()
  if (!trimmed) return null

  if (trimmed.startsWith('blob:') || trimmed.startsWith('data:audio/')) {
    return trimmed
  }

  if (options.allowRawBase64 && isLikelyBase64Payload(trimmed)) {
    const payload = getCleanBase64Payload(trimmed)
    return `data:${getAudioMimeType(payload)};base64,${payload}`
  }

  return null
}

export function buildImageGenerationMarkdown(
  response: ImageGenerationResponse
): string {
  const images = response.data
    ?.map((item) =>
      item.url || (item.b64_json ? `data:image/png;base64,${item.b64_json}` : '')
    )
    .map((url) => url.trim())
    .map(normalizeGeneratedImageUrl)
    .filter(Boolean)

  if (!images?.length) {
    return 'No image returned'
  }

  return images
    .map((url, index) => `![Generated image ${index + 1}](${url})`)
    .join('\n\n')
}

export function buildSpeechGenerationMarkdown(audioUrl: string): string {
  return `[Audio Preview](${audioUrl})`
}

export function extractGeneratedImageUrls(
  content: string,
  options: ExtractGeneratedMediaOptions = {}
): string[] {
  const trimmed = content.trim()

  if (trimmed.startsWith('data:image/')) {
    return [trimmed]
  }

  const markdownImages = [...trimmed.matchAll(generatedImageMarkdownPattern)]
    .map((match) => match[1]?.trim() ?? '')
    .map(normalizeGeneratedImageUrl)
    .filter(Boolean)

  if (markdownImages.length > 0) {
    return markdownImages
  }

  if (options.allowRawBase64 && isLikelyBase64Payload(trimmed)) {
    return [normalizeGeneratedImageUrl(trimmed)]
  }

  return []
}

export function extractGeneratedSpeechUrl(
  content: string,
  options: ExtractGeneratedMediaOptions = {}
): string | null {
  const trimmed = content.trim()

  return (
    normalizeGeneratedSpeechUrl(trimmed, options) ||
    normalizeGeneratedSpeechUrl(
      speechMarkdownPattern.exec(trimmed)?.[1] ?? '',
      options
    )
  )
}
