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
import type { TypeSafeStageSummary } from '../types'

export function formatTypeSafeAnswerValue(answer: unknown): string {
  if (answer == null) return '-'
  if (
    typeof answer === 'string' ||
    typeof answer === 'number' ||
    typeof answer === 'boolean'
  ) {
    return String(answer)
  }
  if (typeof answer !== 'object') {
    return String(answer)
  }
  const record = answer as Record<string, unknown>
  if (record.type === 'choice' && record.choice != null) {
    return String(record.choice)
  }
  if (record.type === 'score' && record.score != null) {
    return String(record.score)
  }
  if (record.type === 'noul' && record.noul != null) {
    const value = Number(record.noul)
    if (Number.isFinite(value)) {
      return value.toFixed(2)
    }
    return String(record.noul)
  }
  for (const key of ['choice', 'score', 'noul', 'value'] as const) {
    if (record[key] != null) {
      return String(record[key])
    }
  }
  try {
    return JSON.stringify(answer)
  } catch {
    return '-'
  }
}

export function listTypeSafeAnswers(
  answers?: Record<string, unknown> | null
): { name: string; value: string }[] {
  if (!answers) return []
  return Object.entries(answers).map(([name, answer]) => ({
    name,
    value: formatTypeSafeAnswerValue(answer),
  }))
}

export function splitTypeSafeStages(results?: TypeSafeStageSummary[] | null): {
  before?: TypeSafeStageSummary
  after?: TypeSafeStageSummary
} {
  let before: TypeSafeStageSummary | undefined
  let after: TypeSafeStageSummary | undefined
  for (const item of results ?? []) {
    if (item.parent_request_id) continue
    if (item.stage === 'before') before = item
    if (item.stage === 'after') after = item
  }
  return { before, after }
}
