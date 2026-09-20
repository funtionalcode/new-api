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
export interface TypeSafeStageSummary {
  model?: string
  stage?: string
  status?: string
  reason?: string
  truncated?: boolean
  answers?: Record<string, unknown>
  parent_request_id?: string
}

export type TypeSafeStage = 'before' | 'after'

export interface TypeSafeEvaluationDetail extends TypeSafeStageSummary {
  stage: TypeSafeStage
  request?: { body: string; bytes?: number; truncated?: boolean }
  response?: { body: string; bytes?: number; truncated?: boolean }
}

export interface TypeSafeUserEvaluation {
  created_at: number
  model_name: string
  token_name: string
  request_id: string
  group?: string
  before?: TypeSafeStageSummary
  after?: TypeSafeStageSummary
}

export interface GetTypeSafeEvaluationsParams {
  p?: number
  page_size?: number
  model_name?: string
  request_id?: string
  start_timestamp?: number
  end_timestamp?: number
}

export interface GetTypeSafeEvaluationsResponse {
  success: boolean
  message?: string
  data?: {
    items: TypeSafeUserEvaluation[]
    total: number
    page: number
    page_size: number
  }
}
