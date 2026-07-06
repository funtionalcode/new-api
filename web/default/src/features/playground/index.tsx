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
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { sendImageGeneration, sendSpeechGeneration } from './api'
import { PlaygroundChat } from './components/chat/playground-chat'
import { PlaygroundInput } from './components/input/playground-input'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
} from './hooks'
import {
  appendUserMessagePair,
  buildImageGenerationMarkdown,
  buildSpeechGenerationMarkdown,
  completeAssistantMessage,
  parseRequestErrorDetails,
  updateAssistantMessageWithError,
  updateCurrentVersionContent,
  updateLastAssistantMessage,
} from './lib'
import type { PlaygroundMode } from './types'

export function Playground() {
  const { t } = useTranslation()
  const [mode, setMode] = useState<PlaygroundMode>('chat')
  const [isTaskGenerating, setIsTaskGenerating] = useState(false)
  const taskAbortControllerRef = useRef<AbortController | null>(null)
  const {
    config,
    parameterEnabled,
    messages,
    isLoadingMessages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    clearMessages,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  const handleTaskSubmit = useCallback(
    async (text: string) => {
      const nextMessages = appendUserMessagePair(messages, text)
      const abortController = new AbortController()
      taskAbortControllerRef.current = abortController

      updateMessages(nextMessages)
      setIsTaskGenerating(true)

      try {
        const content =
          mode === 'image'
            ? buildImageGenerationMarkdown(
                await sendImageGeneration(
                  {
                    model: config.model,
                    group: config.group,
                    prompt: text,
                    n: 1,
                    size: '1024x1024',
                  },
                  abortController.signal
                )
              )
            : buildSpeechGenerationMarkdown(
                URL.createObjectURL(
                  await sendSpeechGeneration(
                    {
                      model: config.model,
                      group: config.group,
                      input: text,
                      voice: 'alloy',
                    },
                    abortController.signal
                  )
                )
              )

        if (abortController.signal.aborted) return

        updateMessages((previousMessages) =>
          updateLastAssistantMessage(previousMessages, (message) =>
            completeAssistantMessage(updateCurrentVersionContent(message, content))
          )
        )
      } catch (error) {
        if (abortController.signal.aborted) return

        const { errorCode, errorMessage } = parseRequestErrorDetails(error)
        toast.error(errorMessage)
        updateMessages((previousMessages) =>
          updateAssistantMessageWithError(
            previousMessages,
            errorMessage,
            errorCode,
            t('Request error occurred')
          )
        )
      } finally {
        if (taskAbortControllerRef.current === abortController) {
          taskAbortControllerRef.current = null
        }
        if (!abortController.signal.aborted) {
          setIsTaskGenerating(false)
        }
      }
    },
    [config.group, config.model, messages, mode, t, updateMessages]
  )

  const {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  } = usePlaygroundConversation({
    messages,
    updateMessages,
    sendChat,
  })

  const handleClearMessages = () => {
    handleEditOpenChange(false)
    clearMessages()
  }

  const { isLoadingModels } = usePlaygroundOptions({
    currentGroup: config.group,
    currentModel: config.model,
    setGroups,
    setModels,
    updateConfig,
  })

  const isBusy = isGenerating || isTaskGenerating

  return (
    <div className='relative flex size-full min-h-0 flex-col overflow-hidden'>
      {/* Full-width scroll container: scrolling works even over side whitespace */}
      <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
        <PlaygroundChat
          messages={messages}
          isLoadingMessages={isLoadingMessages}
          onRegenerateMessage={handleRegenerateMessage}
          onEditMessage={handleEditMessage}
          onDeleteMessage={handleDeleteMessage}
          onSelectPrompt={handleSendMessage}
          isGenerating={isBusy}
          editingKey={editingMessageKey}
          onCancelEdit={handleEditOpenChange}
          onSaveEdit={(newContent) => applyEdit(newContent, false)}
          onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
        />
      </div>

      {/* Input area: center content and constrain to the same container width */}
      <div className='mx-auto w-full max-w-4xl'>
        <PlaygroundInput
          disabled={isBusy}
          groups={groups}
          groupValue={config.group}
          isGenerating={isBusy}
          isModelLoading={isLoadingModels}
          mode={mode}
          modelValue={config.model}
          models={models}
          onGroupChange={(value) => updateConfig('group', value)}
          onClearMessages={handleClearMessages}
          onModeChange={setMode}
          onModelChange={(value) => updateConfig('model', value)}
          onStop={mode === 'chat' ? stopGeneration : undefined}
          onSubmit={mode === 'chat' ? handleSendMessage : handleTaskSubmit}
          hasMessages={messages.length > 0}
        />
      </div>
    </div>
  )
}
