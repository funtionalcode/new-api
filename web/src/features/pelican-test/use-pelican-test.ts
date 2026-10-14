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
import { useMutation } from '@tanstack/react-query'
import { nanoid } from 'nanoid'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { markServerErrorHandled } from '@/lib/handle-server-error'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { assessPelicanHtml, generatePelicanHtml } from './api'
import {
  historyStorageKey,
  pelicanGenerationOptionsSchema,
  readPelicanTestHistory,
  writePelicanTestHistory,
  type PelicanTestAssessment,
  type PelicanTestRating,
  type PelicanTestRecord,
} from './lib'

export function usePelicanTest(userId: number) {
  const { t } = useTranslation()
  const storageKey = historyStorageKey(userId)
  const [history, setHistory] = useState<PelicanTestRecord[]>([])
  const [storageError, setStorageError] = useState('')
  const [ready, setReady] = useState(false)
  const historyRef = useRef<PelicanTestRecord[]>([])
  const mountedRef = useRef(false)
  const controllerRef = useRef<AbortController | null>(null)
  const assessmentControllerRef = useRef<AbortController | null>(null)
  const currentRunRef = useRef<string | null>(null)
  const [currentRecordId, setCurrentRecordId] = useState<string | null>(null)

  const commit = useCallback(
    (next: PelicanTestRecord[]) => {
      historyRef.current = next
      setHistory(next)
      try {
        writePelicanTestHistory(storageKey, next)
        setStorageError('')
      } catch {
        setStorageError(
          'History could not be saved. Keep this page open and copy the results before leaving.'
        )
      }
    },
    [storageKey]
  )

  useEffect(() => {
    mountedRef.current = true
    try {
      const saved = readPelicanTestHistory(storageKey).map((record) =>
        record.status === 'running'
          ? { ...record, status: 'cancelled' as const }
          : record
      )
      historyRef.current = saved
      setHistory(saved)
      setCurrentRecordId(saved[0]?.id ?? null)
      setReady(true)
    } catch {
      setStorageError(
        'Test history could not be loaded. Existing stored data has not been overwritten.'
      )
    }
    return () => {
      mountedRef.current = false
      controllerRef.current?.abort()
      assessmentControllerRef.current?.abort()
    }
  }, [storageKey])

  const generation = useMutation({
    retry: false,
    meta: { errorToast: false },
    mutationFn: async (input: {
      model: string
      group: string
      maxTokens: number
    }) => {
      if (controllerRef.current || !ready || !input.model || !input.group) {
        return
      }
      if (!pelicanGenerationOptionsSchema.safeParse(input).success) return
      const controller = new AbortController()
      controllerRef.current = controller
      const record: PelicanTestRecord = {
        id: nanoid(),
        model: input.model,
        group: input.group,
        createdAt: Date.now(),
        status: 'running',
        content: '',
        maxTokens: input.maxTokens,
        assessments: [],
      }
      currentRunRef.current = record.id
      commit([record, ...historyRef.current])
      setCurrentRecordId(record.id)
      const startedAt = performance.now()
      try {
        const result = await generatePelicanHtml(
          input.model,
          input.group,
          input.maxTokens,
          controller.signal
        )
        if (!mountedRef.current || controller.signal.aborted) return
        let errorMessage: string | undefined
        if (result.truncated) {
          errorMessage = t('The model response was truncated')
        } else if (!result.content) {
          errorMessage = t('The model returned an empty result')
        }
        commit(
          historyRef.current.map((item) =>
            item.id === record.id
              ? {
                  ...item,
                  content: result.content,
                  reasoningContent: result.reasoningContent,
                  finishReason: result.finishReason,
                  status: errorMessage ? 'error' : 'success',
                  durationMs: performance.now() - startedAt,
                  errorMessage,
                }
              : item
          )
        )
        if (!errorMessage) toast.success(t('Pelican test completed'))
      } catch (error) {
        if (!mountedRef.current || controller.signal.aborted) return
        markServerErrorHandled(error)
        commit(
          historyRef.current.map((item) =>
            item.id === record.id
              ? {
                  ...item,
                  status: 'error',
                  durationMs: performance.now() - startedAt,
                  errorMessage: getServerErrorMessage(
                    error,
                    t('Pelican test failed')
                  ),
                }
              : item
          )
        )
      } finally {
        if (controllerRef.current === controller) controllerRef.current = null
        currentRunRef.current = null
      }
    },
  })

  const assessment = useMutation({
    retry: false,
    meta: { errorToast: false },
    mutationFn: async (input: {
      model: string
      group: string
      ids: string[]
    }) => {
      if (
        assessmentControllerRef.current ||
        !ready ||
        !input.model ||
        !input.group
      ) {
        return
      }
      const targets = historyRef.current.filter(
        (record) => input.ids.includes(record.id) && record.status === 'success'
      )
      const controller = new AbortController()
      assessmentControllerRef.current = controller
      try {
        // 顺序评审，避免选中多个历史结果时同时发起大量计费请求。
        for (const record of targets) {
          if (controller.signal.aborted) break
          const result: PelicanTestAssessment = {
            id: nanoid(),
            model: input.model,
            group: input.group,
            createdAt: Date.now(),
            status: 'success',
            content: '',
          }
          try {
            const response = await assessPelicanHtml(
              input.model,
              input.group,
              record,
              controller.signal
            )
            result.content = response.content
            result.reasoningContent = response.reasoningContent
            if (response.truncated) {
              result.status = 'error'
              result.errorMessage = t('The model response was truncated')
            } else if (!response.content) {
              result.status = 'error'
              result.errorMessage = t('The model returned an empty result')
            }
          } catch (error) {
            if (controller.signal.aborted) break
            markServerErrorHandled(error)
            result.status = 'error'
            result.errorMessage = getServerErrorMessage(
              error,
              t('Pelican assessment failed')
            )
          }
          if (!mountedRef.current || controller.signal.aborted) break
          commit(
            historyRef.current.map((item) =>
              item.id === record.id
                ? { ...item, assessments: [result, ...item.assessments] }
                : item
            )
          )
        }
      } finally {
        assessmentControllerRef.current = null
      }
    },
  })

  const stopGeneration = () => {
    controllerRef.current?.abort()
    commit(
      historyRef.current.map((record) =>
        record.id === currentRunRef.current
          ? {
              ...record,
              status: 'cancelled',
              errorMessage: t('Generation was interrupted'),
            }
          : record
      )
    )
  }

  const rate = (id: string, rating: PelicanTestRating) => {
    commit(
      historyRef.current.map((record) =>
        record.id === id ? { ...record, rating } : record
      )
    )
  }

  return {
    history,
    ready,
    storageError,
    currentRecordId,
    setCurrentRecordId,
    generation,
    assessment,
    stopGeneration,
    stopAssessment: () => assessmentControllerRef.current?.abort(),
    rate,
  }
}
