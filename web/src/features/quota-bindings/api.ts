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
  ApiResponse,
  PageData,
  QuotaBinding,
  QuotaBindingSavePayload,
  QuotaProvider,
} from './types'

const providerBasePath: Record<QuotaProvider, string> = {
  glm: '/api/glm-quota',
  deepseek: '/api/deepseek-quota',
  kimi: '/api/kimi-quota',
}

function pathFor(provider: QuotaProvider, suffix: string): string {
  return `${providerBasePath[provider]}${suffix}`
}

export async function getQuotaBindings(
  provider: QuotaProvider,
  params: { keyword?: string; p?: number; page_size?: number } = {}
): Promise<ApiResponse<PageData<QuotaBinding>>> {
  const res = await api.get(pathFor(provider, '/bindings'), { params })
  return res.data
}

export async function createQuotaBinding(
  provider: QuotaProvider,
  data: QuotaBindingSavePayload
): Promise<ApiResponse<QuotaBinding>> {
  const res = await api.post(pathFor(provider, '/bindings'), data)
  return res.data
}

export async function updateQuotaBinding(
  provider: QuotaProvider,
  id: number,
  data: QuotaBindingSavePayload
): Promise<ApiResponse<QuotaBinding>> {
  const res = await api.put(pathFor(provider, `/bindings/${id}`), data)
  return res.data
}

export async function deleteQuotaBinding(
  provider: QuotaProvider,
  id: number
): Promise<ApiResponse<null>> {
  const res = await api.delete(pathFor(provider, `/bindings/${id}`))
  return res.data
}

export async function refreshQuotaBindingUsage(
  provider: QuotaProvider,
  id: number
): Promise<ApiResponse<QuotaBinding>> {
  const res = await api.post(pathFor(provider, `/bindings/${id}/refresh-usage`))
  return res.data
}
