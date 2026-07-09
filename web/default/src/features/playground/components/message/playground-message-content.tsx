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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Loader } from '@/components/ai-elements/loader'
import { MessageContent } from '@/components/ai-elements/message'
import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from '@/components/ai-elements/reasoning'
import { Response } from '@/components/ai-elements/response'
import { Shimmer } from '@/components/ai-elements/shimmer'
import {
  Source,
  Sources,
  SourcesContent,
  SourcesTrigger,
} from '@/components/ai-elements/sources'
import { cn } from '@/lib/utils'

import { MESSAGE_STATUS } from '../../constants'
import {
  getMessageAlignmentClass,
  getMessageContentState,
  isErrorMessage,
  extractGeneratedImageUrls,
  extractGeneratedSpeechUrl,
  extractGeneratedVideoUrl,
  type MessageAlignment,
} from '../../lib'
import { getMessageContentStyles } from '../../lib/message/message-styles'
import type { Message } from '../../types'
import { MessageError } from './message-error'
import { MessageMetadata } from './message-metadata'

type PlaygroundMessageContentProps = {
  actions: ReactNode
  alignment: MessageAlignment
  errorActions?: ReactNode
  isSourceVisible?: boolean
  message: Message
  versionContent: string
}

export function PlaygroundMessageContent({
  actions,
  alignment,
  errorActions,
  isSourceVisible = false,
  message,
  versionContent,
}: PlaygroundMessageContentProps) {
  const { t } = useTranslation()
  const {
    displayContent,
    hasReasoning,
    hasSources,
    reasoningContent,
    showLoader,
    showMessageContent,
    sources,
  } = getMessageContentState(message, versionContent)
  const isError = isErrorMessage(message)
  const generatedImageUrls = extractGeneratedImageUrls(displayContent, {
    allowRawBase64: message.mode === 'image',
  })
  const generatedSpeechUrl = extractGeneratedSpeechUrl(displayContent, {
    allowRawBase64: message.mode === 'speech',
  })
  const generatedVideoUrl =
    message.mode === 'video' ? extractGeneratedVideoUrl(displayContent) : null
  const attachedImageUrls =
    message.from === 'user' ? [...new Set(message.imageUrls ?? [])] : []
  const hasGeneratedMedia =
    generatedImageUrls.length > 0 ||
    generatedSpeechUrl !== null ||
    generatedVideoUrl !== null
  const hasAttachedImages = attachedImageUrls.length > 0
  const isMessageFinal =
    message.status !== MESSAGE_STATUS.LOADING &&
    message.status !== MESSAGE_STATUS.STREAMING

  return (
    <div
      className={cn(
        'flex w-full min-w-0 flex-col',
        getMessageAlignmentClass(alignment)
      )}
    >
      {hasSources && (
        <Sources>
          <SourcesTrigger count={sources.length} />
          <SourcesContent>
            {sources.map((source) => (
              <Source
                href={source.href}
                key={`${source.href}-${source.title}`}
                title={source.title}
              />
            ))}
          </SourcesContent>
        </Sources>
      )}

      {hasReasoning && (
        <Reasoning
          defaultOpen
          duration={message.reasoning?.duration}
          isStreaming={message.isReasoningStreaming}
        >
          <ReasoningTrigger />
          <ReasoningContent>{reasoningContent}</ReasoningContent>
        </Reasoning>
      )}

      {showLoader && (
        <div className='flex items-center gap-2 py-2'>
          <Loader />
          <Shimmer className='text-sm' duration={1}>
            {t('Responding...')}
          </Shimmer>
        </div>
      )}

      {isError && (
        <>
          <MessageError message={message} className='mb-2' />
          <MessageMetadata alignment={alignment} message={message} />
          {errorActions}
        </>
      )}

      {!isError && (showMessageContent || hasAttachedImages) && (
        <>
          {isSourceVisible ? (
            <CodeBlock
              code={versionContent}
              className='my-0 group-[.is-assistant]:w-full group-[.is-assistant]:max-w-[78ch]'
              collapsedLines={24}
              defaultCollapsed={false}
              language='markdown'
              maxExpandedLines={48}
              showLineNumbers
              showToolbar
              title={t('Raw response')}
            >
              <CodeBlockCopyButton />
            </CodeBlock>
          ) : (
            <MessageContent
              variant='flat'
              className={cn(getMessageContentStyles())}
            >
              {hasGeneratedMedia || hasAttachedImages ? (
                <div className='flex flex-col gap-3'>
                  {attachedImageUrls.map((url) => (
                    <img
                      alt={t('Image')}
                      className='border-border/60 max-h-[360px] max-w-full rounded-lg border object-contain'
                      key={url}
                      loading='lazy'
                      src={url}
                    />
                  ))}
                  {generatedImageUrls.map((url, index) => (
                    <img
                      alt={t('Generated image {{index}}', {
                        index: index + 1,
                      })}
                      className='border-border/60 max-h-[640px] max-w-full rounded-lg border object-contain'
                      key={url}
                      loading='lazy'
                      src={url}
                    />
                  ))}
                  {generatedSpeechUrl ? (
                    <audio
                      className='w-full max-w-xl'
                      controls
                      src={generatedSpeechUrl}
                    />
                  ) : null}
                  {generatedVideoUrl ? (
                    <video
                      className='border-border/60 max-h-[640px] max-w-full rounded-lg border'
                      controls
                      src={generatedVideoUrl}
                    />
                  ) : null}
                  {hasAttachedImages && displayContent ? (
                    <Response final={isMessageFinal}>{displayContent}</Response>
                  ) : null}
                </div>
              ) : (
                <Response final={isMessageFinal}>{displayContent}</Response>
              )}
            </MessageContent>
          )}
          <MessageMetadata alignment={alignment} message={message} />
          {actions}
        </>
      )}
    </div>
  )
}
