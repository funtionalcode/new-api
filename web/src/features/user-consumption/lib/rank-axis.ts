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
/** Rough character-unit width: CJK ~2 units, ASCII ~1 unit. */
function labelUnits(label: string): number {
  let maxLine = 0
  for (const line of String(label).split('\n')) {
    let units = 0
    for (const ch of line) {
      units += ch.charCodeAt(0) > 255 ? 2 : 1
    }
    maxLine = Math.max(maxLine, units)
  }
  return maxLine
}

/**
 * Left padding wide enough so horizontal-bar category labels are not clipped.
 * Clamped so short names stay compact and long category labels still fit.
 */
export function estimateRankAxisLeftPadding(labels: string[]): number {
  const maxUnits = labels.reduce(
    (max, label) => Math.max(max, labelUnits(label)),
    0
  )
  // ~7.5px per unit + tick gap; keep within a readable range for the card.
  return Math.min(200, Math.max(80, Math.ceil(maxUnits * 7.5) + 16))
}

/** Band axis config that keeps every ranking label visible (no sampling/hide). */
export function horizontalRankBandAxis() {
  return {
    orient: 'left' as const,
    type: 'band' as const,
    sampling: false,
    label: {
      visible: true,
      autoHide: false,
      autoLimit: false,
      style: {
        fontSize: 11,
        lineHeight: 14,
      },
    },
  }
}

export function horizontalRankChartPadding(labels: string[]) {
  return {
    left: estimateRankAxisLeftPadding(labels),
    // Outside bar value labels (e.g. $12,820.6989) need room on the right.
    right: 64,
    top: 8,
    bottom: 8,
  }
}

/** Chart pixel height so each band has enough room for its label. */
export function rankChartHeight(
  itemCount: number,
  min = 300,
  max = 720
): number {
  if (itemCount <= 0) return min
  // Title/subtext ~56px + ~28px per row keeps labels from overlapping.
  return Math.min(max, Math.max(min, 56 + itemCount * 32))
}
