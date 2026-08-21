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
import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import {
  formatAuditIdentity,
  getManageTargetUserLabel,
} from '../audit-identity'

describe('usage log audit identities', () => {
  test('shows the target username and ID for quota adjustments', () => {
    const label = getManageTargetUserLabel({
      op: {
        action: 'user.quota_add',
        params: {
          quota: '$50.000000 额度',
          target_user_id: 42,
          target_username: 'quota-recipient',
        },
      },
    })

    assert.equal(label, 'quota-recipient (ID: 42)')
  })

  test('supports historical user audit params without dedicated target fields', () => {
    const label = getManageTargetUserLabel({
      op: {
        action: 'user.update',
        params: { id: 7, username: 'legacy-user' },
      },
    })

    assert.equal(label, 'legacy-user (ID: 7)')
  })

  test('does not present resource audit params as a target user', () => {
    const label = getManageTargetUserLabel({
      op: {
        action: 'channel.update',
        params: { id: 9, username: 'not-a-user' },
      },
    })

    assert.equal(label, null)
  })

  test('formats an ID when a username is unavailable', () => {
    assert.equal(formatAuditIdentity(undefined, 21), 'ID: 21')
  })
})
