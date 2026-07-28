import { LOG_TYPE_ENUM } from '@/features/usage-logs/constants'

import type { ModelStatusErrorDetail } from '../types'

export const MODEL_STATUS_ERROR_LOG_TYPE = String(LOG_TYPE_ENUM.ERROR)

/** 错误发生时间前后各放宽 5 分钟，避免边界截断。 */
export const MODEL_STATUS_ERROR_TIME_WINDOW_MS = 5 * 60 * 1000

export type ModelStatusErrorUsageLogSearch = {
  model: string
  type: [typeof MODEL_STATUS_ERROR_LOG_TYPE]
  page: 1
  requestId?: string
  startTime?: number
  endTime?: number
}

export function buildModelStatusErrorUsageLogSearch(input: {
  modelName: string
  detail: ModelStatusErrorDetail
}): ModelStatusErrorUsageLogSearch {
  const modelName = input.modelName.trim()
  const requestId = input.detail.request_id?.trim()
  const search: ModelStatusErrorUsageLogSearch = {
    model: modelName,
    type: [MODEL_STATUS_ERROR_LOG_TYPE],
    page: 1,
  }

  if (requestId) {
    search.requestId = requestId
  }

  // 始终带时间窗，避免落到 usage-logs 默认“今天”范围而漏掉近 24 小时内的历史错误。
  if (input.detail.created_at > 0) {
    const createdAtMs = input.detail.created_at * 1000
    search.startTime = Math.max(
      0,
      createdAtMs - MODEL_STATUS_ERROR_TIME_WINDOW_MS
    )
    search.endTime = createdAtMs + MODEL_STATUS_ERROR_TIME_WINDOW_MS
  }

  return search
}
