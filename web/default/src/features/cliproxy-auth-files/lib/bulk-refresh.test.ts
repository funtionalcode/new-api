import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { refreshCliproxyAuthFileBindingsUsageAll } from './bulk-refresh'
import type { CliproxyAuthFileBinding } from '../types'

describe('cliproxy auth file bulk refresh', () => {
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
})

function createBinding(id: number, enabled: boolean): CliproxyAuthFileBinding {
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
    last_xai_on_demand_cap: 0,
    last_xai_billing_period_end_at: 0,
    last_error: '',
    created_at: 0,
    updated_at: 0,
  }
}
