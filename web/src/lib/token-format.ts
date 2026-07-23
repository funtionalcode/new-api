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

const exactTokenUnitLabels = new Map<number, string>([
  [100, '百'],
  [1000, '千'],
  [10_000, '万'],
  [1_000_000, '百万'],
  [10_000_000, '千万'],
  [100_000_000, '亿'],
])

function formatTokenScaledValue(value: number): string {
  const floored = Math.floor(value * 10) / 10
  return Number.isInteger(floored) ? String(floored) : floored.toFixed(1)
}

/**
 * Format token counts with compact Chinese units.
 */
export function formatTokens(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return '0'

  const normalizedTokens = Math.floor(tokens)
  const exactLabel = exactTokenUnitLabels.get(normalizedTokens)
  if (exactLabel) return exactLabel

  if (normalizedTokens >= 100_000_000) {
    return `${formatTokenScaledValue(normalizedTokens / 100_000_000)}亿`
  }
  if (normalizedTokens >= 10_000) {
    return `${formatTokenScaledValue(normalizedTokens / 10_000)}万`
  }
  if (normalizedTokens >= 1000) {
    return `${formatTokenScaledValue(normalizedTokens / 1000)}千`
  }
  if (normalizedTokens >= 100 && normalizedTokens % 100 === 0) {
    return `${normalizedTokens / 100}百`
  }
  return normalizedTokens.toString()
}

/**
 * Format exact token counts for tooltip details.
 */
export function formatTokenDetails(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return '0 token'
  return `${Math.floor(tokens).toLocaleString()} token`
}
