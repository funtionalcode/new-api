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

import { useAuthStore } from '@/stores/auth-store'

import {
  getVideoGeneration,
  sendImageGeneration,
  sendSpeechGeneration,
  sendVideoGeneration,
} from './api'
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
  buildVideoGenerationMarkdown,
  completeAssistantMessage,
  getMessageContent,
  getPreviousUserMessage,
  getPlaygroundGenerationMode,
  parseRequestErrorDetails,
  getPlaygroundTaskModel,
  pollVideoGeneration,
  updateAssistantMessageWithError,
  updateCurrentVersionContent,
  updateLastAssistantMessage,
} from './lib'
import type { Message, PlaygroundMode } from './types'

const VIDEO_POLL_INTERVAL_MS = 2000

function waitForVideoPollInterval(signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(resolve, VIDEO_POLL_INTERVAL_MS)
    signal.addEventListener(
      'abort',
      () => {
        window.clearTimeout(timeout)
        reject(signal.reason ?? new DOMException('Aborted', 'AbortError'))
      },
      { once: true }
    )
  })
}

export function Playground() {
  const { t } = useTranslation()
  const userId = useAuthStore((state) => state.auth.user?.id)
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
    updateParameterEnabled,
    clearMessages,
  } = usePlaygroundState(userId)

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  const sendTaskMessages = useCallback(
    async (
      taskMessages: Message[],
      taskMode: Exclude<PlaygroundMode, 'chat'>
    ) => {
      const promptMessage = getPreviousUserMessage(
        taskMessages,
        taskMessages.length
      )
      const text = promptMessage ? getMessageContent(promptMessage).trim() : ''
      const imageUrls = promptMessage?.imageUrls ?? []
      if (!text) return

      const abortController = new AbortController()
      taskAbortControllerRef.current = abortController

      setIsTaskGenerating(true)

      try {
        let content = ''
        if (taskMode === 'image') {
          const imageModel = getPlaygroundTaskModel('image', config.model)
          content = buildImageGenerationMarkdown(
            await sendImageGeneration(
              {
                model: imageModel,
                group: config.group,
                prompt: text,
                n: 1,
                size: '1024x1024',
              },
              abortController.signal
            )
          )
        } else if (taskMode === 'video') {
          const videoModel = getPlaygroundTaskModel('video', config.model)
          const submittedVideo = await sendVideoGeneration(
            {
              model: videoModel,
              group: config.group,
              prompt: text,
              image: imageUrls[0],
              duration: 6,
            },
            abortController.signal
          )
          const videoId = submittedVideo.id || submittedVideo.task_id
          if (!videoId) {
            throw new Error(t('No video task returned'))
          }
          const video = await pollVideoGeneration({
            videoId,
            initialVideo: submittedVideo,
            signal: abortController.signal,
            failureMessage: t('Video generation failed'),
            getVideoGeneration,
            waitForPollInterval: waitForVideoPollInterval,
          })
          content = buildVideoGenerationMarkdown(
            video.metadata?.url || `/v1/videos/${videoId}/content`
          )
        } else {
          content = buildSpeechGenerationMarkdown(
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
        }

        if (abortController.signal.aborted) return

        updateMessages((previousMessages) =>
          updateLastAssistantMessage(previousMessages, (message) =>
            completeAssistantMessage(
              updateCurrentVersionContent(message, content)
            )
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
    [config.group, config.model, t, updateMessages]
  )

  const stopTaskGeneration = useCallback(() => {
    taskAbortControllerRef.current?.abort()
    taskAbortControllerRef.current = null
    setIsTaskGenerating(false)
  }, [])

  const sendMessages = useCallback(
    (nextMessages: Message[], generationMode: PlaygroundMode = 'chat') => {
      const effectiveMode = getPlaygroundGenerationMode(
        generationMode,
        config.model
      )

      if (effectiveMode === 'chat') {
        sendChat(nextMessages)
        return
      }

      void sendTaskMessages(nextMessages, effectiveMode)
    },
    [config.model, sendChat, sendTaskMessages]
  )

  const handleSubmitMessage = useCallback(
    (text: string, imageUrls: string[] = []) => {
      const nextMessages = appendUserMessagePair(
        messages,
        text,
        mode,
        imageUrls
      )
      updateMessages(nextMessages)
      sendMessages(nextMessages, mode)
    },
    [messages, mode, sendMessages, updateMessages]
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
    sendMessages,
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
          config={config}
          disabled={isBusy}
          groups={groups}
          groupValue={config.group}
          isGenerating={isBusy}
          isModelLoading={isLoadingModels}
          mode={mode}
          modelValue={config.model}
          models={models}
          onGroupChange={(value) => updateConfig('group', value)}
          onConfigChange={updateConfig}
          onClearMessages={handleClearMessages}
          onModeChange={setMode}
          onModelChange={(value) => updateConfig('model', value)}
          onParameterEnabledChange={updateParameterEnabled}
          onStop={mode === 'chat' ? stopGeneration : stopTaskGeneration}
          onSubmit={handleSubmitMessage}
          parameterEnabled={parameterEnabled}
          hasMessages={messages.length > 0}
        />
      </div>
    </div>
  )
}
