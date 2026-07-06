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

export function buildImageGenerationMarkdown(
  response: ImageGenerationResponse
): string {
  const images = response.data
    ?.map((item) => item.url || (item.b64_json ? `data:image/png;base64,${item.b64_json}` : ''))
    .map((url) => url.trim())
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
