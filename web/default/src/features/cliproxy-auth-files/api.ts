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
  CliproxyAuthFile,
  CliproxyAuthFileBinding,
  CliproxyAuthFileBindingFormData,
  GetCliproxyAuthFileBindingsParams,
  GetCliproxyAuthFileBindingsResponse,
  GetCliproxyRemoteAuthFilesResponse,
} from './types'

export async function getCliproxyRemoteAuthFiles(): Promise<GetCliproxyRemoteAuthFilesResponse> {
  const res = await api.get('/api/cliproxy/auth-files/remote')
  return res.data
}

export async function getCliproxyAuthFileBindings(
  params: GetCliproxyAuthFileBindingsParams = {}
): Promise<GetCliproxyAuthFileBindingsResponse> {
  const res = await api.get('/api/cliproxy/auth-files/bindings', { params })
  return res.data
}

export async function createCliproxyAuthFileBinding(
  data: CliproxyAuthFileBindingFormData
): Promise<ApiResponse<CliproxyAuthFileBinding>> {
  const res = await api.post('/api/cliproxy/auth-files/bindings', data)
  return res.data
}

export async function updateCliproxyAuthFileBinding(
  id: number,
  data: CliproxyAuthFileBindingFormData
): Promise<ApiResponse<CliproxyAuthFileBinding>> {
  const res = await api.put(`/api/cliproxy/auth-files/bindings/${id}`, data)
  return res.data
}

export async function deleteCliproxyAuthFileBinding(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/cliproxy/auth-files/bindings/${id}`)
  return res.data
}

export async function refreshCliproxyAuthFileBindingUsage(
  id: number
): Promise<ApiResponse<CliproxyAuthFileBinding>> {
  const res = await api.post(
    `/api/cliproxy/auth-files/bindings/${id}/refresh-usage`
  )
  return res.data
}

export function toBindingFormData(
  authFile: CliproxyAuthFile,
  userId: number
): CliproxyAuthFileBindingFormData {
  return {
    user_id: userId,
    auth_index: authFile.authIndex,
    auth_name: authFile.name,
    auth_file: authFile.authFile || '',
    description: '',
    account_id: authFile.accountId || '',
    enabled: authFile.enabled,
  }
}
