import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type {
  CliproxyAuthFileBinding,
  GetCliproxyAuthFileBindingsResponse,
} from '../../cliproxy-auth-files/types'
import { fetchAllXAIQuotaBindings } from './xai-bindings'

function successResponse(
  page: number,
  total: number,
  ids: number[]
): GetCliproxyAuthFileBindingsResponse {
  return {
    success: true,
    data: {
      items: ids.map((id) => ({ id }) as CliproxyAuthFileBinding),
      page,
      page_size: 100,
      total,
    },
  }
}

describe('xAI quota binding pagination', () => {
  test('limits later page requests while preserving every page order', async () => {
    const requestedPages: number[] = []
    const pendingPageResolvers = new Map<
      number,
      (response: GetCliproxyAuthFileBindingsResponse) => void
    >()
    let inFlight = 0
    let maxInFlight = 0
    let firstBatchStarted: (() => void) | undefined
    const firstBatch = new Promise<void>((resolve) => {
      firstBatchStarted = resolve
    })
    let pageSixStarted: (() => void) | undefined
    const pageSix = new Promise<void>((resolve) => {
      pageSixStarted = resolve
    })
    let pageSevenStarted: (() => void) | undefined
    const pageSeven = new Promise<void>((resolve) => {
      pageSevenStarted = resolve
    })

    const resultPromise = fetchAllXAIQuotaBindings((params) => {
      const page = params.p ?? 1
      requestedPages.push(page)
      if (page === 1) {
        return Promise.resolve(successResponse(1, 700, [1]))
      }

      inFlight += 1
      maxInFlight = Math.max(maxInFlight, inFlight)
      if (inFlight === 4) {
        firstBatchStarted?.()
      }
      if (page === 6) {
        pageSixStarted?.()
      }
      if (page === 7) {
        pageSevenStarted?.()
      }
      return new Promise((resolve) => {
        pendingPageResolvers.set(page, (response) => {
          inFlight -= 1
          resolve(response)
        })
      })
    })

    await firstBatch
    assert.deepEqual(requestedPages, [1, 2, 3, 4, 5])
    assert.equal(maxInFlight, 4)

    pendingPageResolvers.get(2)?.(successResponse(2, 700, [2]))
    await pageSix
    pendingPageResolvers.get(3)?.(successResponse(3, 700, [3]))
    await pageSeven
    assert.equal(maxInFlight, 4)

    pendingPageResolvers.get(7)?.(successResponse(7, 700, [7]))
    pendingPageResolvers.get(6)?.(successResponse(6, 700, [6]))
    pendingPageResolvers.get(5)?.(successResponse(5, 700, [5]))
    pendingPageResolvers.get(4)?.(successResponse(4, 700, [4]))

    const result = await resultPromise
    assert.equal(result.success, true)
    assert.deepEqual(
      result.data?.items.map((binding) => binding.id),
      [1, 2, 3, 4, 5, 6, 7]
    )
    assert.equal(result.data?.total, 700)
  })

  test('returns any later page api failure for the quota page error state', async () => {
    const result = await fetchAllXAIQuotaBindings((params) => {
      if (params.p === 1) {
        return Promise.resolve(successResponse(1, 401, [1]))
      }
      if (params.p === 4) {
        return Promise.resolve({
          success: false,
          message: 'page 4 failed',
        })
      }
      return Promise.resolve(
        successResponse(params.p ?? 1, 401, [params.p ?? 1])
      )
    })

    assert.equal(result.success, false)
    assert.equal(result.message, 'page 4 failed')
  })
})
