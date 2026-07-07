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
import type { FlowQuotaDataItem } from '@/features/dashboard/types'
import { formatTokens } from '@/lib/format'

type TFunction = (key: string, options?: Record<string, unknown>) => string

const TOKEN_RANK_COLORS = [
  '#5B8FF9',
  '#5AD8A6',
  '#F6BD16',
  '#E8684A',
  '#6DC8EC',
  '#9270CA',
  '#FF9D4D',
  '#269A99',
  '#FF99C3',
  '#5D7092',
]

function formatTokenValue(value: number): string {
  return value <= 0 ? '0' : formatTokens(value)
}

function emptyTokenRankSpec(t: TFunction) {
  return {
    type: 'bar',
    data: [{ id: 'userTokenRankData', values: [] }],
    xField: 'rawValue',
    yField: 'Token',
    seriesField: 'Token',
    direction: 'horizontal',
    title: {
      visible: true,
      text: t('Token Consumption Ranking'),
      subtext: t('No data available'),
    },
    legends: { visible: false },
    color: { type: 'ordinal', range: TOKEN_RANK_COLORS },
    background: { fill: 'transparent' },
  }
}

export function processUserTokenRankChartData(
  data: FlowQuotaDataItem[],
  t?: TFunction,
  limit = 10
) {
  const translate = t ?? ((key) => key)
  const emptySpec = emptyTokenRankSpec(translate)

  if (!data || data.length === 0) return emptySpec

  const tokenTotals = new Map<string, { label: string; tokens: number }>()
  for (const item of data) {
    const tokenID = Number(item.token_id) || 0
    const tokenName = item.token_name?.trim()
    if (tokenID <= 0 && !tokenName) continue

    const key = tokenID > 0 ? `token:${tokenID}` : `token:${tokenName}`
    const label =
      tokenName ||
      (tokenID > 0 ? translate('Deleted ({{id}})', { id: tokenID }) : '')
    if (!label) continue

    const existing = tokenTotals.get(key) || { label, tokens: 0 }
    existing.tokens += Number(item.token_used) || 0
    tokenTotals.set(key, existing)
  }

  const sorted = [...tokenTotals.values()]
    .filter((item) => item.tokens > 0)
    .sort((a, b) => b.tokens - a.tokens)
  if (sorted.length === 0) return emptySpec

  const visibleItems = sorted.slice(0, limit)
  const totalValue = visibleItems.reduce((sum, item) => sum + item.tokens, 0)
  const rankValues = visibleItems.map((item) => ({
    Token: item.label,
    rawValue: item.tokens,
  }))
  const tokenColorMap = rankValues.reduce<Record<string, string>>(
    (acc, item, index) => {
      acc[item.Token] = TOKEN_RANK_COLORS[index % TOKEN_RANK_COLORS.length]
      return acc
    },
    {}
  )

  return {
    type: 'bar',
    data: [{ id: 'userTokenRankData', values: rankValues }],
    xField: 'rawValue',
    yField: 'Token',
    seriesField: 'Token',
    direction: 'horizontal',
    title: {
      visible: true,
      text: translate('Token Consumption Ranking'),
      subtext: `${translate('Total:')} ${formatTokenValue(totalValue)}`,
    },
    legends: { visible: false },
    bar: {
      state: { hover: { stroke: '#000', lineWidth: 1 } },
    },
    label: {
      visible: true,
      position: 'outside',
      formatMethod: (value: number) => formatTokenValue(value),
      style: { fontSize: 11 },
    },
    axes: [
      { orient: 'left', type: 'band' },
      {
        orient: 'bottom',
        type: 'linear',
        visible: false,
        title: { visible: false, text: translate('Tokens') },
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum?.Token,
            value: (datum: Record<string, unknown>) =>
              formatTokenValue(Number(datum?.rawValue) || 0),
          },
        ],
      },
    },
    color: { specified: tokenColorMap },
    background: { fill: 'transparent' },
    animation: true,
  }
}
