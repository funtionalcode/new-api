export type ModelAvailabilityStatus = 'normal' | 'warning' | 'error' | 'no_request'

export type ModelStatusOverview = {
  normal_count: number
  warning_count: number
  error_count: number
  model_count: number
  request_count: number
  avg_success_rate: number
  updated_at: number
  window_start: number
  window_end: number
  auto_refresh_seconds: number
}

export type ModelStatusToday = {
  tokens: number
  quota: number
  request_count: number
  rpm: number
  tpm: number
  started_at: number
}

export type ModelStatusBucket = {
  start: number
  request_count: number
  success_rate: number
  status: ModelAvailabilityStatus
  error_details?: ModelStatusErrorDetail[]
}

export type ModelStatusErrorDetail = {
  created_at: number
  message: string
  status_code?: number
  error_type?: string
  error_code?: string
  request_id?: string
}

export type ModelStatusModel = {
  model_name: string
  request_count: number
  token_count: number
  quota: number
  success_rate: number
  avg_first_response_time_ms: number
  avg_latency_ms: number
  tps: number
  status: ModelAvailabilityStatus
  error_details?: ModelStatusErrorDetail[]
  buckets: ModelStatusBucket[]
}

export type ModelStatusSnapshot = {
  overview: ModelStatusOverview
  today: ModelStatusToday
  models: ModelStatusModel[]
}
