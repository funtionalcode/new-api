import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import type { CliproxyAuthFileBinding } from '../types'
import {
  getCliproxyAuthFileBulkRefreshOptions,
  refreshCliproxyAuthFileBindingsUsageAll,
  refreshCliproxyAuthFileBindingsUsageByType,
} from './bulk-refresh'

describe('cliproxy auth file bulk refresh', () => {
  test('offers bulk refresh only for enabled Codex and Claude bindings', () => {
    const options = getCliproxyAuthFileBulkRefreshOptions([
      createBinding(1, true),
      createBinding(2, false),
      createBinding(3, true, { auth_name: 'claude-a@example.com.json' }),
      createBinding(4, true, { auth_name: 'claude-b@example.com.json' }),
      createBinding(5, true, {
        auth_name: 'xai-a@example.com.json',
        last_plan_type: 'supergrokheavy',
      }),
    ])

    assert.deepEqual(options, [
      { type: 'codex', count: 1 },
      { type: 'claude', count: 2 },
    ])
  })

  test('refreshes enabled bindings and skips disabled ones', async () => {
    const refreshedIds: number[] = []

    const summary = await refreshCliproxyAuthFileBindingsUsageAll(
      [createBinding(1, true), createBinding(2, false), createBinding(3, true)],
      async (id) => {
        refreshedIds.push(id)
        return {
          success: true,
          data: createBinding(id, true),
        }
      }
    )

    assert.deepEqual(refreshedIds, [1, 3])
    assert.deepEqual(summary, {
      total: 2,
      success: 2,
      failed: 0,
    })
  })

  test('counts api failures and saved last errors as failed refreshes', async () => {
    const summary = await refreshCliproxyAuthFileBindingsUsageAll(
      [createBinding(1, true), createBinding(2, true), createBinding(3, true)],
      async (id) => {
        if (id === 1) {
          return {
            success: true,
            data: createBinding(id, true),
          }
        }
        if (id === 2) {
          return {
            success: false,
            message: 'upstream failed',
          }
        }
        return {
          success: true,
          data: {
            ...createBinding(id, true),
            last_error: 'invalid response',
          },
        }
      }
    )

    assert.deepEqual(summary, {
      total: 3,
      success: 1,
      failed: 2,
    })
  })

  test('refreshes only bindings matching the requested auth file type', async () => {
    const refreshedIds: number[] = []

    const summary = await refreshCliproxyAuthFileBindingsUsageByType(
      [
        createBinding(1, true, { auth_name: 'codex-a@example.com.json' }),
        createBinding(2, true, { auth_name: 'claude-b@example.com.json' }),
        createBinding(3, true, {
          auth_name: 'xai-c@example.com.json',
          last_plan_type: 'supergrokheavy',
        }),
        createBinding(4, false, {
          auth_name: 'xai-disabled@example.com.json',
          last_plan_type: 'supergrokheavy',
        }),
      ],
      'xai',
      async (id) => {
        refreshedIds.push(id)
        return {
          success: true,
          data: createBinding(id, true),
        }
      }
    )

    assert.deepEqual(refreshedIds, [3])
    assert.deepEqual(summary, {
      total: 1,
      success: 1,
      failed: 0,
    })
  })

  test('limits concurrency while refreshing all bindings', async () => {
    let inFlight = 0
    let maxInFlight = 0
    const summary = await refreshCliproxyAuthFileBindingsUsageAll(
      [
        createBinding(1, true),
        createBinding(2, true),
        createBinding(3, true),
        createBinding(4, true),
      ],
      async (id) => {
        inFlight++
        maxInFlight = Math.max(maxInFlight, inFlight)
        await new Promise((resolve) => setTimeout(resolve, 20))
        inFlight--
        return {
          success: true,
          data: createBinding(id, true),
        }
      },
      2
    )

    assert.equal(summary.success, 4)
    assert.equal(summary.failed, 0)
    assert.ok(maxInFlight <= 2)
  })
})

function createBinding(
  id: number,
  enabled: boolean,
  patch: Partial<CliproxyAuthFileBinding> = {}
): CliproxyAuthFileBinding {
  return {
    id,
    user_id: 1,
    username: 'root',
    remark: '',
    auth_index: `auth-${id}`,
    auth_name: `codex-user-${id}@example.com.json`,
    auth_file: '',
    description: '',
    account_id: '',
    enabled,
    last_refreshed_at: 0,
    last_usage_tokens: 0,
    last_usage_quota: 0,
    last_plan_type: '',
    last_five_hour_percent: 0,
    last_five_hour_reset_at: 0,
    last_weekly_percent: 0,
    last_weekly_reset_at: 0,
    last_codex_five_hour_percent: 0,
    last_codex_five_hour_reset_at: 0,
    last_codex_weekly_percent: 0,
    last_codex_weekly_reset_at: 0,
    last_claude_fable_percent: 0,
    last_claude_fable_reset_at: 0,
    last_xai_weekly_percent: 0,
    last_xai_weekly_period_start_at: 0,
    last_xai_weekly_period_end_at: 0,
    last_xai_product_usage: '',
    last_xai_on_demand_cap: 0,
    last_xai_on_demand_used: 0,
    last_xai_billing_period_end_at: 0,
    last_error: '',
    created_at: 0,
    updated_at: 0,
    ...patch,
  }
}
