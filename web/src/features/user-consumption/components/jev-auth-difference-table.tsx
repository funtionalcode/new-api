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
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePagination,
  DataTableView,
  useDataTable,
} from '@/components/data-table'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import {
  compareJevAuthUsage,
  type JevAuthComparison,
  type JevComparisonMetrics,
} from '../lib/jev-comparison'
import type { JevDailyUsageRow } from '../types'
import { JevComparisonValue, JevDifferenceValue } from './jev-difference-value'

export function JevAuthDifferenceTable(props: {
  baseline: JevDailyUsageRow[]
  current: JevDailyUsageRow[]
}) {
  const { t } = useTranslation()
  const baseline = props.baseline
  const current = props.current
  const data = useMemo(
    () => compareJevAuthUsage(baseline, current),
    [baseline, current]
  )
  const columns = useMemo<ColumnDef<JevAuthComparison>[]>(() => {
    const metrics: {
      key: keyof JevComparisonMetrics
      label: string
      money?: boolean
    }[] = [
      { key: 'request_count', label: t('Main requests') },
      { key: 'used_requests', label: t('Jev used') },
      { key: 'prompt_tokens', label: t('Main input tokens') },
      { key: 'completion_tokens', label: t('Main output tokens') },
      { key: 'average_output_tokens', label: t('Output tokens per request') },
      { key: 'evaluation_quota', label: t('Jev evaluation cost'), money: true },
      { key: 'combined_quota', label: t('Combined cost'), money: true },
      {
        key: 'average_combined_quota',
        label: t('Combined cost per request'),
        money: true,
      },
    ]
    return [
      {
        id: 'auth',
        header: t('Auth file'),
        size: 250,
        cell: ({ row }) => (
          <div className='space-y-1'>
            <p
              className='max-w-64 truncate font-medium'
              title={
                row.original.auth.auth_name || row.original.auth.auth_index
              }
            >
              {row.original.auth.auth_name ||
                row.original.auth.auth_index ||
                t('Unknown')}
            </p>
            <p className='text-muted-foreground text-xs'>
              {row.original.auth.username || `#${row.original.auth.user_id}`} ·{' '}
              {row.original.auth.auth_index}
            </p>
          </div>
        ),
      },
      ...metrics.map(
        (metric): ColumnDef<JevAuthComparison> => ({
          id: metric.key,
          header: metric.label,
          size: 180,
          cell: ({ row }) => (
            <div className='space-y-1'>
              <div className='text-muted-foreground flex items-center gap-1 text-xs'>
                <JevComparisonValue
                  value={row.original.baseline[metric.key]}
                  money={metric.money}
                />
                <span aria-label={t('Baseline to analysis')}>→</span>
                <JevComparisonValue
                  value={row.original.current[metric.key]}
                  money={metric.money}
                />
              </div>
              <JevDifferenceValue
                baseline={row.original.baseline[metric.key]}
                current={row.original.current[metric.key]}
                money={metric.money}
              />
            </div>
          ),
        })
      ),
    ]
  }, [t])
  const { table } = useDataTable({
    data,
    columns,
    initialPagination: { pageIndex: 0, pageSize: 10 },
    withPaginationRowModel: true,
    getRowId: (row) => row.key,
    enableRowSelection: false,
  })
  return (
    <section aria-label={t('Auth file differences')}>
      <Card>
        <CardHeader>
          <CardTitle>{t('Auth file differences')}</CardTitle>
          <CardDescription>
            {t(
              'Each cell shows baseline → analysis, followed by the difference and percentage. Files appearing on only one day are retained.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-3'>
          <DataTableView
            table={table}
            tableClassName='min-w-[1700px]'
            emptyTitle={t('No Data')}
          />
          <DataTablePagination table={table} />
        </CardContent>
      </Card>
    </section>
  )
}
