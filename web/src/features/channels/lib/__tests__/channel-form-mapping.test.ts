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
import { expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  buildSettingJSON,
  transformChannelToFormDefaults,
} from '../channel-form'

test('editing a channel preserves cross-channel mappings through its settings', () => {
  const channel = channelSchema.parse({
    id: 7,
    type: 1,
    key: '',
    status: 1,
    name: 'DeepSeek',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    model_mapping: '{"deepseek-v4-flash":"qwen3.8-27b"}',
    setting: '{"model_mapping_channels":{"deepseek-v4-flash":12}}',
    channel_info: { is_multi_key: false },
  })
  const form = transformChannelToFormDefaults(channel)
  expect(form.model_mapping_channels).toEqual({ 'deepseek-v4-flash': 12 })
  expect(JSON.parse(buildSettingJSON(form)).model_mapping_channels).toEqual({
    'deepseek-v4-flash': 12,
  })
  expect(form.model_mapping).toBe('{"deepseek-v4-flash":"qwen3.8-27b"}')
  expect(
    JSON.parse(buildSettingJSON({ ...form, model_mapping_channels: {} }))
      .model_mapping_channels
  ).toBeUndefined()
  expect(
    JSON.parse(buildSettingJSON({ ...form, model_mapping: '{}' }))
      .model_mapping_channels
  ).toBeUndefined()
})
