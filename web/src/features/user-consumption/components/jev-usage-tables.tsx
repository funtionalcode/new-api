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
  DataTableColumnHeader,
  DataTablePagination,
  DataTableView,
  StaticDataTable,
  useDataTable,
} from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { toIntlLocale } from '@/i18n/languages'
import { formatLogQuota, formatNumber } from '@/lib/format'

import {
  groupJevUsageByAuth,
  jevStatuses,
  sumJevUsage,
  type JevAuthTotal,
} from '../lib/jev-daily'
import type { JevDailyUsageRow, JevUsageStatus } from '../types'

function useJevStatusLabels(): Record<JevUsageStatus, string> {
  const { t } = useTranslation()
  return {
    used: t('Jev used'),
    not_used: t('No Jev recorded'),
    not_applied: t('Jev skipped or not applied'),
    unknown: t('Unknown Jev status'),
    evaluation_only: t('Standalone or unlinked Jev'),
    no_activity: t('No activity'),
  }
}

export function JevCohortTable(props: {
  rows: JevDailyUsageRow[]
  title: string
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const labels = useJevStatusLabels()
  const cohorts = jevStatuses
    .filter((status) => status !== 'no_activity')
    .map((status) => ({
      status,
      ...sumJevUsage(props.rows.filter((row) => row.jev_status === status)),
    }))
  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.title}</CardTitle>
      </CardHeader>
      <CardContent>
        <StaticDataTable
          tableProps={{ 'aria-label': props.title }}
          tableClassName='min-w-[900px]'
          data={cohorts}
          getRowKey={(row) => row.status}
          columns={[
            {
              id: 'cohort',
              header: t('Jev status'),
              cell: (row) => labels[row.status],
            },
            {
              id: 'requests',
              header: t('Main requests'),
              cell: (row) => formatNumber(row.request_count, locale),
            },
            {
              id: 'output',
              header: t('Output tokens per request'),
              cell: (row) =>
                row.request_count
                  ? formatNumber(
                      row.completion_tokens / row.request_count,
                      locale
                    )
                  : '-',
            },
            {
              id: 'main',
              header: t('Main model cost'),
              cell: (row) => formatLogQuota(row.quota),
            },
            {
              id: 'jev',
              header: t('Jev evaluation cost'),
              cell: (row) => formatLogQuota(row.evaluation_quota),
            },
            {
              id: 'combined',
              header: t('Combined cost'),
              cell: (row) => formatLogQuota(row.quota + row.evaluation_quota),
            },
            {
              id: 'average',
              header: t('Combined cost per request'),
              cell: (row) =>
                row.request_count
                  ? formatLogQuota(
                      (row.quota + row.evaluation_quota) / row.request_count
                    )
                  : '-',
            },
            {
              id: 'reuse',
              header: t('Reused decisions'),
              cell: (row) => formatNumber(row.cached_decisions, locale),
            },
          ]}
        />
      </CardContent>
    </Card>
  )
}

export function JevAuthUsageTable(props: {
  rows: JevDailyUsageRow[]
  title: string
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const data = useMemo(() => groupJevUsageByAuth(props.rows), [props.rows])
  const columns = useMemo<ColumnDef<JevAuthTotal>[]>(
    () => [
      {
        id: 'auth',
        accessorKey: 'auth_name',
        header: t('Auth file'),
        size: 260,
        cell: ({ row }) => (
          <div className='space-y-1'>
            <p
              className='max-w-64 truncate font-medium'
              title={row.original.auth_name || row.original.auth_index}
            >
              {row.original.auth_name ||
                row.original.auth_index ||
                t('Unknown')}
            </p>
            <p className='text-muted-foreground text-xs'>
              {row.original.username || `#${row.original.user_id}`} ·{' '}
              {row.original.auth_index}
            </p>
            {row.original.provider && (
              <Badge variant='outline'>{row.original.provider}</Badge>
            )}
            {!row.original.bound && (
              <Badge variant='outline'>{t('No matching auth binding')}</Badge>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'used_requests',
        header: t('Jev used'),
        cell: ({ row }) => formatNumber(row.original.used_requests, locale),
      },
      {
        accessorKey: 'not_used_requests',
        header: t('No Jev recorded'),
        cell: ({ row }) => formatNumber(row.original.not_used_requests, locale),
      },
      {
        accessorKey: 'not_applied_requests',
        header: t('Jev skipped or not applied'),
        cell: ({ row }) =>
          formatNumber(row.original.not_applied_requests, locale),
      },
      {
        accessorKey: 'unknown_requests',
        header: t('Unknown Jev status'),
        cell: ({ row }) => formatNumber(row.original.unknown_requests, locale),
      },
      {
        accessorKey: 'prompt_tokens',
        header: t('Main input tokens'),
        cell: ({ row }) => formatNumber(row.original.prompt_tokens, locale),
      },
      {
        accessorKey: 'completion_tokens',
        header: t('Main output tokens'),
        cell: ({ row }) => formatNumber(row.original.completion_tokens, locale),
      },
      {
        accessorKey: 'quota',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Main model cost')} />
        ),
        cell: ({ row }) => formatLogQuota(row.original.quota),
      },
      {
        accessorKey: 'evaluation_requests',
        header: t('Jev evaluations'),
        cell: ({ row }) =>
          formatNumber(row.original.evaluation_requests, locale),
      },
      {
        id: 'evaluation_tokens',
        header: t('Jev evaluation tokens'),
        cell: ({ row }) =>
          formatNumber(
            row.original.evaluation_prompt_tokens +
              row.original.evaluation_completion_tokens,
            locale
          ),
      },
      {
        accessorKey: 'evaluation_quota',
        header: t('Jev evaluation cost'),
        cell: ({ row }) => formatLogQuota(row.original.evaluation_quota),
      },
      {
        id: 'total',
        accessorFn: (row) => row.quota + row.evaluation_quota,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Combined cost')} />
        ),
        cell: ({ row }) =>
          formatLogQuota(row.original.quota + row.original.evaluation_quota),
      },
    ],
    [locale, t]
  )
  const { table } = useDataTable({
    data,
    columns,
    initialPagination: { pageIndex: 0, pageSize: 10 },
    withPaginationRowModel: true,
    withSortedRowModel: true,
    getRowId: (row) => row.key,
    enableRowSelection: false,
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.title}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3'>
        <DataTableView
          table={table}
          tableClassName='min-w-[1400px]'
          emptyTitle={t('No Data')}
        />
        <DataTablePagination table={table} />
      </CardContent>
    </Card>
  )
}
