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
import { t } from 'i18next'
import { SSE } from 'sse.js'

import { API_ENDPOINTS, ERROR_MESSAGES } from '@/features/playground/constants'
import { createStreamRequestController } from '@/features/playground/hooks/use-stream-request'
import { parseThinkTags } from '@/features/playground/lib/message/message-reasoning-utils'
import type { ChatCompletionRequest } from '@/features/playground/types'
import { getFreshAuthHeaders } from '@/lib/api'

export type ModelStreamResponse = {
  content: string
  reasoningContent: string
  truncated: boolean
  finishReason?: string
}

function readModelResponse(
  rawContent: string,
  rawReasoning: string,
  finishReason?: string
): ModelStreamResponse {
  // 只处理响应开头的思考标签，避免误拆 HTML 源码中的同名元素或字符串。
  const trimmed = rawContent.trimStart()
  let content = rawContent
  let reasoning = ''
  if (trimmed && '<think>'.startsWith(trimmed)) {
    content = ''
  } else if (trimmed.startsWith('<think>')) {
    const end = rawContent.indexOf('</think>')
    const prefix = end === -1 ? rawContent : rawContent.slice(0, end + 8)
    reasoning = parseThinkTags(prefix).reasoning
    content = end === -1 ? '' : rawContent.slice(end + 8)
  }
  return {
    content: content.trim(),
    reasoningContent: rawReasoning.trim() || reasoning,
    truncated: finishReason === 'length',
    finishReason,
  }
}

export function streamModelResponse(
  payload: ChatCompletionRequest,
  signal: AbortSignal,
  onUpdate?: (response: ModelStreamResponse) => void
): Promise<ModelStreamResponse> {
  return new Promise((resolve, reject) => {
    let rawContent = ''
    let rawReasoning = ''
    let finishReason: string | undefined
    let updateTimer: number | null = null
    let idleTimer: number | null = null
    const clearIdleTimeout = () => {
      if (idleTimer !== null) window.clearTimeout(idleTimer)
      idleTimer = null
    }
    const flush = () => {
      if (updateTimer !== null) window.clearTimeout(updateTimer)
      updateTimer = null
      onUpdate?.(readModelResponse(rawContent, rawReasoning, finishReason))
    }
    const controller = createStreamRequestController({
      getHeaders: getFreshAuthHeaders,
      createSource: (request, headers) =>
        new SSE(API_ENDPOINTS.CHAT_COMPLETIONS, {
          headers,
          method: 'POST',
          payload: JSON.stringify(request),
          start: false,
          autoReconnect: false,
          withCredentials: true,
        }),
      setStreaming: () => {},
    })
    const resetIdleTimeout = () => {
      clearIdleTimeout()
      // 上游心跳只能证明连接存活；仅正文或思考增量才代表生成有进展。
      idleTimer = window.setTimeout(() => {
        idleTimer = null
        flush()
        controller.stop()
        signal.removeEventListener('abort', abort)
        reject(
          new Error(
            t(
              'No new output or reasoning received for 5 minutes. Partial results were preserved. Retry manually.'
            )
          )
        )
      }, 300_000)
    }
    const abort = () => {
      clearIdleTimeout()
      flush()
      controller.stop()
      signal.removeEventListener('abort', abort)
      reject(new DOMException('Generation was interrupted', 'AbortError'))
    }
    if (signal.aborted) {
      abort()
      return
    }
    signal.addEventListener('abort', abort, { once: true })
    resetIdleTimeout()
    void controller
      .send(
        payload,
        {},
        {
          onUpdate: (type, chunk) => {
            resetIdleTimeout()
            if (type === 'reasoning') rawReasoning += chunk
            else rawContent += chunk
            if (onUpdate && updateTimer === null) {
              updateTimer = window.setTimeout(flush, 50)
            }
          },
          onFinishReason: (reason) => {
            finishReason = reason
          },
          onComplete: () => {
            clearIdleTimeout()
            flush()
            signal.removeEventListener('abort', abort)
            const result = readModelResponse(
              rawContent,
              rawReasoning,
              finishReason
            )
            if (!result.content && !result.reasoningContent) {
              reject(new Error(t('The model returned an empty result')))
              return
            }
            resolve(result)
          },
          onError: (message) => {
            clearIdleTimeout()
            flush()
            signal.removeEventListener('abort', abort)
            const known = Object.values(ERROR_MESSAGES).some(
              (value) => value === message
            )
            reject(new Error(known ? t(message) : message))
          },
        }
      )
      .catch((error: unknown) => {
        clearIdleTimeout()
        flush()
        signal.removeEventListener('abort', abort)
        controller.dispose()
        reject(error)
      })
  })
}
