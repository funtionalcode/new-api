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
  estimateRankAxisLeftPadding,
  horizontalRankBandAxis,
  horizontalRankChartPadding,
  rankChartHeight,
} from '../rank-axis'

describe('user consumption rank axis helpers', () => {
  test('reserves wider left padding for long category labels', () => {
    const shortPad = estimateRankAxisLeftPadding(['a', 'bb'])
    const longPad = estimateRankAxisLeftPadding([
      'macbook-pro-workstation',
      'liyonghui',
      'magic_xiaodao',
    ])
    assert.ok(longPad > shortPad)
    assert.ok(longPad >= 80)
    assert.ok(longPad <= 200)
  })

  test('keeps every band label visible', () => {
    const axis = horizontalRankBandAxis()
    assert.equal(axis.sampling, false)
    assert.equal(axis.label.autoHide, false)
    assert.equal(axis.label.autoLimit, false)
  })

  test('grows chart height with item count', () => {
    assert.equal(rankChartHeight(0), 300)
    assert.ok(rankChartHeight(15) > rankChartHeight(5))
    assert.ok(rankChartHeight(100) <= 720)
  })

  test('includes right padding for outside bar labels', () => {
    const padding = horizontalRankChartPadding(['token-a'])
    assert.ok(padding.right >= 48)
    assert.ok(padding.left >= 80)
  })
})
