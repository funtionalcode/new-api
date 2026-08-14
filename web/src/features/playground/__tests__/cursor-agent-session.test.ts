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
import { describe, test } from 'node:test'

import { cursorAgentRequestHeaders } from '../api'
import { CURSOR_AGENT_HEADERS } from '../constants'
import type { CursorAgentSession } from '../types'

const session: CursorAgentSession = {
  agentId: 'bc-00000000-0000-0000-0000-000000000001',
  signature: `v2.${'a'.repeat(64)}`,
  channelId: 17,
  keyIndex: 0,
  model: 'grok-4.5',
  group: 'vip',
}

describe('Cursor Agent request session', () => {
  test('reuses the saved channel when model and group both match', () => {
    assert.deepEqual(cursorAgentRequestHeaders(session, 'grok-4.5', 'vip'), {
      [CURSOR_AGENT_HEADERS.PERSISTENT]: 'true',
      [CURSOR_AGENT_HEADERS.ID]: session.agentId,
      [CURSOR_AGENT_HEADERS.SIGNATURE]: session.signature,
      [CURSOR_AGENT_HEADERS.CHANNEL_ID]: String(session.channelId),
      [CURSOR_AGENT_HEADERS.KEY_INDEX]: String(session.keyIndex),
    })
  })

  test('does not force the saved channel after switching group', () => {
    assert.deepEqual(
      cursorAgentRequestHeaders(session, 'grok-4.5', 'default'),
      {
        [CURSOR_AGENT_HEADERS.PERSISTENT]: 'true',
      }
    )
  })

  test('does not force the saved channel after switching model', () => {
    assert.deepEqual(
      cursorAgentRequestHeaders(session, 'claude-sonnet-4', 'vip'),
      {
        [CURSOR_AGENT_HEADERS.PERSISTENT]: 'true',
      }
    )
  })
})
