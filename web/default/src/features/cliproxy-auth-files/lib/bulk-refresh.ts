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
import type { ApiResponse, CliproxyAuthFileBinding } from '../types'

export type CliproxyAuthFileBulkRefreshSummary = {
  total: number
  success: number
  failed: number
}

type RefreshCliproxyAuthFileBindingUsage = (
  id: number
) => Promise<ApiResponse<CliproxyAuthFileBinding>>

export async function refreshCliproxyAuthFileBindingsUsageAll(
  bindings: CliproxyAuthFileBinding[],
  refreshUsage: RefreshCliproxyAuthFileBindingUsage
): Promise<CliproxyAuthFileBulkRefreshSummary> {
  const enabledBindings = bindings.filter((binding) => binding.enabled)
  const results = await Promise.allSettled(
    enabledBindings.map((binding) => refreshUsage(binding.id))
  )

  let success = 0
  let failed = 0
  for (const result of results) {
    if (
      result.status === 'fulfilled' &&
      result.value.success &&
      !result.value.data?.last_error
    ) {
      success++
    } else {
      failed++
    }
  }

  return {
    total: enabledBindings.length,
    success,
    failed,
  }
}
