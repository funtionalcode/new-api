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
import { describe, expect, test } from 'vitest'

import {
  formatTypeSafeAnswerValue,
  listTypeSafeAnswers,
  splitTypeSafeStages,
} from '../lib/answers'

describe('formatTypeSafeAnswerValue', () => {
  test('formats choice score and noul answers', () => {
    expect(
      formatTypeSafeAnswerValue({ type: 'choice', choice: 'coding' })
    ).toBe('coding')
    expect(formatTypeSafeAnswerValue({ type: 'score', score: 2 })).toBe('2')
    expect(formatTypeSafeAnswerValue({ type: 'noul', noul: 0.9 })).toBe('0.90')
  })

  test('returns dash for empty values', () => {
    expect(formatTypeSafeAnswerValue(null)).toBe('-')
  })
})

describe('listTypeSafeAnswers', () => {
  test('lists each question name and formatted value', () => {
    expect(
      listTypeSafeAnswers({
        category: { type: 'choice', choice: 'coding' },
        check: { type: 'noul', noul: 0.9 },
      })
    ).toEqual([
      { name: 'category', value: 'coding' },
      { name: 'check', value: '0.90' },
    ])
  })
})

describe('splitTypeSafeStages', () => {
  test('keeps parent before and after answers and skips child logs', () => {
    const { before, after } = splitTypeSafeStages([
      {
        stage: 'before',
        status: 'success',
        answers: { category: { type: 'choice', choice: 'coding' } },
      },
      {
        stage: 'after',
        parent_request_id: 'parent-1',
        answers: { leaked: true },
      },
      {
        stage: 'after',
        status: 'success',
        answers: { relevance: { type: 'score', score: 2 } },
      },
    ])
    expect(before?.answers?.category).toEqual({
      type: 'choice',
      choice: 'coding',
    })
    expect(after?.answers?.relevance).toEqual({ type: 'score', score: 2 })
  })
})
