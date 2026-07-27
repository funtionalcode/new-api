import { api } from '@/lib/api'

import type { ModelStatusSnapshot } from './types'

type ModelStatusResponse = {
  success: boolean
  message?: string
  data: ModelStatusSnapshot
}

export async function getModelStatus(): Promise<ModelStatusResponse> {
  const res = await api.get('/api/model-status')
  const payload = res.data as ModelStatusResponse
  if (!payload.success) {
    throw new Error(payload.message || 'Unable to load model status')
  }
  return payload
}
