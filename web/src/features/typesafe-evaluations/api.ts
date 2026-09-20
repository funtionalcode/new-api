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
import { api } from '@/lib/api'

import type {
  GetTypeSafeEvaluationsParams,
  GetTypeSafeEvaluationsResponse,
} from './types'

export async function getTypeSafeEvaluations(
  params: GetTypeSafeEvaluationsParams = {}
): Promise<GetTypeSafeEvaluationsResponse> {
  const search = new URLSearchParams()
  search.set('p', String(params.p ?? 1))
  search.set('page_size', String(params.page_size ?? 20))
  if (params.model_name) search.set('model_name', params.model_name)
  if (params.request_id) search.set('request_id', params.request_id)
  if (params.start_timestamp) {
    search.set('start_timestamp', String(params.start_timestamp))
  }
  if (params.end_timestamp) {
    search.set('end_timestamp', String(params.end_timestamp))
  }
  const res = await api.get(`/api/log/self/typesafe?${search.toString()}`)
  return res.data
}
