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

import {
  CHANNEL_TYPE_CURSOR,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon, getKeyPromptForType } from '../channel-utils'

describe('Cursor channel registration', () => {
  test('registers Cursor as a fetchable Cloud Agents channel', () => {
    assert.deepEqual(
      CHANNEL_TYPE_OPTIONS.find((item) => item.value === CHANNEL_TYPE_CURSOR),
      { value: CHANNEL_TYPE_CURSOR, label: 'Cursor' }
    )
    assert.equal(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_CURSOR), true)
    assert.equal(getChannelTypeIcon(CHANNEL_TYPE_CURSOR), 'Cursor')
    assert.equal(getKeyPromptForType(CHANNEL_TYPE_CURSOR), 'Cursor API Key')
    assert.equal(
      getChannelTypeConfig(CHANNEL_TYPE_CURSOR).defaultBaseUrl,
      'https://api.cursor.com'
    )
  })
})
