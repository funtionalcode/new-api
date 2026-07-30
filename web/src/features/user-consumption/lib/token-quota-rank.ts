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
import { formatLogQuota } from '@/lib/format'
import type { UserConsumptionSummary } from '../types'

type TFunction = (key: string, options?: Record<string, unknown>) => string

const QUOTA_RANK_COLORS = [
  '#E8684A',
  '#F6BD16',
  '#5B8FF9',
  '#5AD8A6',
  '#9270CA',
  '#FF9D4D',
  '#6DC8EC',
  '#269A99',
  '#FF99C3',
  '#5D7092',
]

function emptyQuotaRankSpec(t: TFunction) {
  return {
    type: 'bar',
    data: [{ id: 'tokenQuotaRankData', values: [] as Array<Record<string, unknown>> }],
    xField: 'rawValue',
    yField: 'Token',
    seriesField: 'Token',
    direction: 'horizontal',
    title: {
      visible: true,
      text: t('Quota Consumption Ranking'),
      subtext: t('No data available'),
    },
    legends: { visible: false },
    color: { type: 'ordinal', range: QUOTA_RANK_COLORS },
    background: { fill: 'transparent' },
  }
}

export function processTokenQuotaRankChartData(
  data: UserConsumptionSummary[],
  t?: TFunction,
  limit = 15
) {
  const translate = t ?? ((key) => key)
  const emptySpec = emptyQuotaRankSpec(translate)

  if (!data || data.length === 0) return emptySpec

  const tokenTotals = new Map<
    string,
    {
      label: string
      quota: number
      tokens: number
      requests: number
      userIds: Set<number>
    }
  >()

  for (const item of data) {
    const tokenID = Number(item.token_id) || 0
    const tokenName = item.token_name?.trim()
    if (tokenID <= 0 && !tokenName) continue

    const key = tokenID > 0 ? `token:${tokenID}` : `token:${tokenName}`
    const label =
      tokenName ||
      (tokenID > 0 ? translate('Deleted ({{id}})', { id: tokenID }) : '')
    if (!label) continue

    const existing = tokenTotals.get(key) || {
      label,
      quota: 0,
      tokens: 0,
      requests: 0,
      userIds: new Set<number>(),
    }
    existing.quota += Number(item.quota) || 0
    existing.tokens += Number(item.total_tokens) || 0
    existing.requests += Number(item.request_count) || 0
    if (item.user_id > 0) existing.userIds.add(item.user_id)
    tokenTotals.set(key, existing)
  }

  const sorted = [...tokenTotals.values()]
    .filter((item) => item.quota > 0)
    .sort((a, b) => b.quota - a.quota)
  if (sorted.length === 0) return emptySpec

  const visibleItems = sorted.slice(0, limit)
  const totalValue = sorted.reduce((sum, item) => sum + item.quota, 0)
  const rankValues = visibleItems.map((item) => ({
    Token: item.label,
    rawValue: item.quota,
    tokens: item.tokens,
    requests: item.requests,
    userCount: item.userIds.size,
  }))
  const colorMap = rankValues.reduce<Record<string, string>>(
    (acc, item, index) => {
      acc[item.Token] = QUOTA_RANK_COLORS[index % QUOTA_RANK_COLORS.length]
      return acc
    },
    {}
  )

  return {
    type: 'bar',
    data: [{ id: 'tokenQuotaRankData', values: rankValues }],
    xField: 'rawValue',
    yField: 'Token',
    seriesField: 'Token',
    direction: 'horizontal',
    title: {
      visible: true,
      text: translate('Quota Consumption Ranking'),
      subtext: `${translate('Total:')} ${formatLogQuota(totalValue)}`,
    },
    legends: { visible: false },
    bar: {
      state: { hover: { stroke: '#000', lineWidth: 1 } },
    },
    label: {
      visible: true,
      position: 'outside',
      formatMethod: (value: number) => formatLogQuota(value),
      style: { fontSize: 11 },
    },
    axes: [
      { orient: 'left', type: 'band' },
      {
        orient: 'bottom',
        type: 'linear',
        visible: false,
        title: { visible: false, text: translate('Quota') },
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum?.Token,
            value: (datum: Record<string, unknown>) =>
              formatLogQuota(Number(datum?.rawValue) || 0),
          },
          {
            key: translate('Tokens'),
            value: (datum: Record<string, unknown>) =>
              Number(datum?.tokens || 0).toLocaleString(),
          },
          {
            key: translate('Requests'),
            value: (datum: Record<string, unknown>) =>
              Number(datum?.requests || 0).toLocaleString(),
          },
          {
            key: translate('Users'),
            value: (datum: Record<string, unknown>) =>
              Number(datum?.userCount || 0).toLocaleString(),
          },
        ],
      },
    },
    color: { specified: colorMap },
    background: { fill: 'transparent' },
    animation: true,
  }
}
