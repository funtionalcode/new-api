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

import { prepareRevealedChannelKeyForEditing } from '../channel-key-editing'

describe('channel key editing', () => {
  test('keeps every revealed key and selects replacement for multi-key edits', () => {
    const keys = ['cursor-key-one', 'cursor-key-two', 'cursor-key-three'].join(
      '\n'
    )

    assert.deepEqual(prepareRevealedChannelKeyForEditing(keys, true), {
      key: keys,
      keyMode: 'replace',
    })
  })

  test('does not add multi-key update semantics to a single key', () => {
    assert.deepEqual(
      prepareRevealedChannelKeyForEditing('cursor-key-one', false),
      {
        key: 'cursor-key-one',
        keyMode: undefined,
      }
    )
  })
})
