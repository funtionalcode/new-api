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

/** Keep bulk refresh gentle so xAI dual billing calls do not stampede Cliproxy. */
export const CLIPROXY_BULK_REFRESH_CONCURRENCY = 2

export async function refreshCliproxyAuthFileBindingsUsageAll(
  bindings: CliproxyAuthFileBinding[],
  refreshUsage: RefreshCliproxyAuthFileBindingUsage,
  concurrency = CLIPROXY_BULK_REFRESH_CONCURRENCY
): Promise<CliproxyAuthFileBulkRefreshSummary> {
  const enabledBindings = bindings.filter((binding) => binding.enabled)
  const results = await mapWithConcurrency(
    enabledBindings,
    Math.max(1, concurrency),
    (binding) => refreshUsage(binding.id)
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

async function mapWithConcurrency<T, R>(
  items: T[],
  concurrency: number,
  mapper: (item: T) => Promise<R>
): Promise<PromiseSettledResult<R>[]> {
  if (items.length === 0) {
    return []
  }

  const results: PromiseSettledResult<R>[] = new Array(items.length)
  let nextIndex = 0

  const workers = Array.from(
    { length: Math.min(concurrency, items.length) },
    async () => {
      while (true) {
        const current = nextIndex
        nextIndex += 1
        if (current >= items.length) {
          return
        }
        try {
          const value = await mapper(items[current])
          results[current] = { status: 'fulfilled', value }
        } catch (reason) {
          results[current] = { status: 'rejected', reason }
        }
      }
    }
  )

  await Promise.all(workers)
  return results
}
