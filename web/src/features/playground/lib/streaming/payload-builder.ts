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
import type {
  ChatCompletionRequest,
  Message,
  PlaygroundConfig,
  ParameterEnabled,
} from '../../types'
import { formatMessageForAPI, isValidMessage } from '../message/message-utils'

const GLM_5_3_DEFAULT_MAX_TOKENS = 65536
const SONNET_5_DEFAULT_MAX_TOKENS = 128000
const GROK_4_7_CHAT_INSTRUCTIONS =
  'You are a helpful chat assistant. Answer the user directly in this conversation. When asked to create code or a document, include the complete requested content in your response, not just a plan or an announcement. You do not have access to a terminal, filesystem, or tools.'

/**
 * Build API request payload from messages and config
 */
export function buildChatCompletionPayload(
  messages: Message[],
  config: PlaygroundConfig,
  parameterEnabled: ParameterEnabled
): ChatCompletionRequest {
  // Filter and format valid messages
  const processedMessages = messages
    .filter(isValidMessage)
    .map(formatMessageForAPI)
  const normalizedModel = config.model.trim().toLowerCase()

  if (
    (normalizedModel === 'grok-4.7' ||
      normalizedModel.startsWith('grok-4.7-')) &&
    !processedMessages.some((message) => message.role === 'system')
  ) {
    // 普通聊天需要直接返回产物；避免 Grok 4.7 只给出准备说明后结束。
    processedMessages.unshift({
      role: 'system',
      content: GROK_4_7_CHAT_INSTRUCTIONS,
    })
  }

  const payload: ChatCompletionRequest = {
    model: config.model,
    group: config.group,
    messages: processedMessages,
    stream: config.stream,
  }

  if (parameterEnabled.temperature) {
    payload.temperature = config.temperature
  }

  if (parameterEnabled.top_p) {
    payload.top_p = config.top_p
  }

  if (parameterEnabled.max_tokens) {
    payload.max_tokens = config.max_tokens
  } else {
    if (
      normalizedModel === 'glm-5.3' ||
      normalizedModel.startsWith('glm-5.3-')
    ) {
      payload.max_tokens = GLM_5_3_DEFAULT_MAX_TOKENS
    } else if (
      normalizedModel === 'claude-sonnet-5' ||
      normalizedModel.startsWith('claude-sonnet-5-')
    ) {
      // Claude 转换要求 max_tokens；自动模式显式发送模型上限，避免回落到 8192。
      payload.max_tokens = SONNET_5_DEFAULT_MAX_TOKENS
    }
  }

  if (parameterEnabled.frequency_penalty) {
    payload.frequency_penalty = config.frequency_penalty
  }

  if (parameterEnabled.presence_penalty) {
    payload.presence_penalty = config.presence_penalty
  }

  if (parameterEnabled.seed && config.seed !== null) {
    payload.seed = config.seed
  }

  return payload
}
