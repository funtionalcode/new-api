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

import { channelSchema } from '../../types'
import {
  buildSettingJSON,
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
} from '../channel-form'

describe('Cursor Agent channel settings', () => {
  test('loads serialized Agent execution from the channel setting', () => {
    const channel = channelSchema.parse({
      id: 63,
      type: 63,
      key: '',
      status: 1,
      name: 'Cursor pool',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      setting: JSON.stringify({ cursor_agent_serial_execution: true }),
      channel_info: {
        is_multi_key: true,
        multi_key_size: 2,
        multi_key_polling_index: 0,
        multi_key_mode: 'polling',
      },
    })

    const defaults = transformChannelToFormDefaults(channel)

    assert.equal(defaults.cursor_agent_serial_execution, true)
  })

  test('saves serialized Agent execution in the channel setting', () => {
    const setting = JSON.parse(
      buildSettingJSON({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        cursor_agent_serial_execution: true,
      })
    ) as Record<string, unknown>

    assert.equal(setting.cursor_agent_serial_execution, true)
  })
})
