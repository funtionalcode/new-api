import { MESSAGE_ROLES } from '../../constants'
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
import type { Message, PlaygroundMode } from '../../types'
import {
  createLoadingAssistantMessage,
  createUserMessage,
  getMessageContent,
  updateCurrentVersionContent,
} from './message-utils'

type ApplyMessageEditResult = {
  messages: Message[]
  shouldSend: boolean
}

type ChatMessageRenderState = {
  alwaysShowActions: boolean
  content: string
  isEditing: boolean
}

function getMessageMode(message?: Message | null): PlaygroundMode {
  return message?.mode ?? 'chat'
}

function getRegenerationMode(
  messages: Message[],
  messageIndex: number
): PlaygroundMode {
  const message = messages[messageIndex]
  if (message?.mode) {
    return message.mode
  }

  if (message?.from === MESSAGE_ROLES.ASSISTANT) {
    return getMessageMode(getPreviousUserMessage(messages, messageIndex))
  }

  return 'chat'
}

export function appendUserMessagePair(
  messages: Message[],
  content: string,
  mode: PlaygroundMode = 'chat',
  imageUrls: string[] = []
): Message[] {
  const submittedAt = Date.now()

  return [
    ...messages,
    createUserMessage(content, submittedAt, mode, imageUrls),
    createLoadingAssistantMessage(submittedAt, mode),
  ]
}

export function createRegeneratedMessages(
  messages: Message[],
  messageKey: string
): Message[] | null {
  const messageIndex = messages.findIndex(
    (message) => message.key === messageKey
  )

  if (messageIndex === -1) {
    return null
  }

  const mode = getRegenerationMode(messages, messageIndex)

  if (messages[messageIndex].from === MESSAGE_ROLES.USER) {
    return [
      ...messages.slice(0, messageIndex + 1),
      createLoadingAssistantMessage(undefined, mode),
    ]
  }

  return [
    ...messages.slice(0, messageIndex),
    createLoadingAssistantMessage(undefined, mode),
  ]
}

export function removeMessageByKey(
  messages: Message[],
  messageKey: string
): Message[] {
  return messages.filter((message) => message.key !== messageKey)
}

export function getPreviousUserMessage(
  messages: Message[],
  beforeIndex: number
): Message | null {
  for (let index = beforeIndex - 1; index >= 0; index--) {
    if (messages[index].from === MESSAGE_ROLES.USER) {
      return messages[index]
    }
  }

  return null
}

export function getPendingGenerationMode(messages: Message[]): PlaygroundMode {
  const lastMessage = messages.at(-1)
  if (lastMessage?.from === MESSAGE_ROLES.ASSISTANT) {
    return (
      lastMessage.mode ??
      getMessageMode(getPreviousUserMessage(messages, messages.length))
    )
  }

  const previousUserMessage = getPreviousUserMessage(messages, messages.length)
  return getMessageMode(previousUserMessage)
}

export function applyMessageEdit(
  messages: Message[],
  messageKey: string,
  content: string,
  shouldSubmit: boolean
): ApplyMessageEditResult | null {
  const submittedAt = Date.now()
  const messageIndex = messages.findIndex(
    (message) => message.key === messageKey
  )

  if (messageIndex === -1) {
    return null
  }

  const updatedMessages = messages.map((message) =>
    message.key === messageKey
      ? {
          ...updateCurrentVersionContent(message, content),
          createdAt: shouldSubmit ? submittedAt : message.createdAt,
        }
      : message
  )

  if (
    !shouldSubmit ||
    updatedMessages[messageIndex].from !== MESSAGE_ROLES.USER
  ) {
    return { messages: updatedMessages, shouldSend: false }
  }

  const mode = getMessageMode(updatedMessages[messageIndex])

  return {
    messages: [
      ...updatedMessages.slice(0, messageIndex + 1),
      createLoadingAssistantMessage(submittedAt, mode),
    ],
    shouldSend: true,
  }
}

export function getEditingMessageContent(
  messages: Message[],
  editingKey?: string | null
): string {
  if (!editingKey) {
    return ''
  }

  const message = messages.find((item) => item.key === editingKey)
  return message ? getMessageContent(message) : ''
}

export function getChatMessageRenderState(
  messages: Message[],
  message: Message,
  messageIndex: number,
  editingKey?: string | null
): ChatMessageRenderState {
  return {
    alwaysShowActions:
      messageIndex === messages.length - 1 &&
      message.from === MESSAGE_ROLES.ASSISTANT,
    content: getMessageContent(message),
    isEditing: editingKey === message.key,
  }
}
