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
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import dayjs from '@/lib/dayjs'

import {
  getJevComparisonMetrics,
  type JevComparisonMetrics,
} from '../lib/jev-comparison'
import { sumJevUsage } from '../lib/jev-daily'
import type { JevDailyReport } from '../types'
import { JevComparisonValue, JevDifferenceValue } from './jev-difference-value'

export function JevDifferenceSummary(props: {
  baseline: JevDailyReport
  current: JevDailyReport
}) {
  const { t } = useTranslation()
  const before = getJevComparisonMetrics(
    sumJevUsage(props.baseline.rows),
    props.baseline.rows.reduce(
      (total, row) =>
        total + (row.jev_status === 'used' ? row.request_count : 0),
      0
    )
  )
  const after = getJevComparisonMetrics(
    sumJevUsage(props.current.rows),
    props.current.rows.reduce(
      (total, row) =>
        total + (row.jev_status === 'used' ? row.request_count : 0),
      0
    )
  )
  const metrics: {
    key: keyof JevComparisonMetrics
    label: string
    money?: boolean
  }[] = [
    { key: 'request_count', label: t('Main requests') },
    { key: 'used_requests', label: t('Jev used') },
    { key: 'prompt_tokens', label: t('Main input tokens') },
    { key: 'completion_tokens', label: t('Main output tokens') },
    { key: 'average_input_tokens', label: t('Input tokens per request') },
    { key: 'average_output_tokens', label: t('Output tokens per request') },
    { key: 'evaluation_tokens', label: t('Jev evaluation tokens') },
    { key: 'quota', label: t('Main model cost'), money: true },
    { key: 'evaluation_quota', label: t('Jev evaluation cost'), money: true },
    { key: 'combined_quota', label: t('Combined cost'), money: true },
    {
      key: 'average_combined_quota',
      label: t('Combined cost per request'),
      money: true,
    },
  ]
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Day-to-day difference')}</CardTitle>
        <CardDescription>
          {t(
            'Difference = analysis day minus baseline day; percentage uses the baseline. Per-request cost includes Jev evaluations. No reasoning-level prices are required.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <StaticDataTable
          tableProps={{ 'aria-label': t('Day-to-day difference') }}
          tableClassName='min-w-[700px]'
          data={metrics}
          getRowKey={(row) => row.key}
          columns={[
            { id: 'metric', header: t('Metric'), cell: (row) => row.label },
            {
              id: 'baseline',
              header: `${t('Baseline date')} · ${dayjs.unix(props.baseline.start_timestamp).format('YYYY-MM-DD')}`,
              cell: (row) => (
                <JevComparisonValue value={before[row.key]} money={row.money} />
              ),
            },
            {
              id: 'current',
              header: `${t('Analysis date')} · ${dayjs.unix(props.current.start_timestamp).format('YYYY-MM-DD')}`,
              cell: (row) => (
                <JevComparisonValue value={after[row.key]} money={row.money} />
              ),
            },
            {
              id: 'difference',
              header: t('Difference / change (%)'),
              cell: (row) => (
                <JevDifferenceValue
                  baseline={before[row.key]}
                  current={after[row.key]}
                  money={row.money}
                />
              ),
            },
          ]}
        />
      </CardContent>
    </Card>
  )
}
