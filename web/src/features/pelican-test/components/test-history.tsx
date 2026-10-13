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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePagination, useDataTable } from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { toIntlLocale } from '@/i18n/languages'
import { cn } from '@/lib/utils'

import type { PelicanTestRecord } from '../lib'

export function TestHistory(props: {
  records: PelicanTestRecord[]
  currentId: string | null
  selectedIds: string[]
  onSelect: (id: string) => void
  onView: (id: string) => void
}) {
  const { t, i18n } = useTranslation()
  const [modelFilter, setModelFilter] = useState('')
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 10 })
  const filtered = useMemo(
    () =>
      props.records.filter(
        (record) => !modelFilter || record.model === modelFilter
      ),
    [props.records, modelFilter]
  )
  const models = useMemo(
    () => [...new Set(props.records.map((record) => record.model))],
    [props.records]
  )
  const { table } = useDataTable({
    data: filtered,
    columns: [],
    totalCount: filtered.length,
    manualPagination: true,
    pagination,
    onPaginationChange: setPagination,
    columnFilters: [],
    columnVisibilityStorageKey: false,
    columnSizingStorageKey: false,
  })
  const rows = filtered.slice(
    pagination.pageIndex * pagination.pageSize,
    (pagination.pageIndex + 1) * pagination.pageSize
  )
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Test history')}</CardTitle>
        <CardDescription>
          {t('Select up to four runs to compare model output and quality.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-3'>
        <NativeSelect
          aria-label={t('Filter by model')}
          value={modelFilter}
          onChange={(event) => {
            setModelFilter(event.target.value)
            setPagination((previous) => ({ ...previous, pageIndex: 0 }))
          }}
        >
          <NativeSelectOption value=''>{t('All Models')}</NativeSelectOption>
          {models.map((model) => (
            <NativeSelectOption key={model} value={model}>
              {model}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        {rows.length === 0 ? (
          <EmptyState
            bordered
            className='min-h-32'
            title={t('No Pelican tests yet')}
          />
        ) : (
          <div className='space-y-2'>
            {rows.map((record) => {
              const selected = props.selectedIds.includes(record.id)
              let status = t('Completed')
              if (record.status === 'error') status = t('Failed')
              if (record.status === 'running') status = t('Generating...')
              if (record.status === 'cancelled') {
                status = t('Generation was interrupted')
              }
              return (
                <div
                  key={record.id}
                  className={cn(
                    'flex min-w-0 items-center gap-3 rounded-lg border p-3',
                    selected && 'border-primary bg-primary/5'
                  )}
                >
                  <Checkbox
                    checked={selected}
                    onCheckedChange={() => props.onSelect(record.id)}
                    disabled={!selected && props.selectedIds.length >= 4}
                    aria-label={t('Select test run {{model}}', {
                      model: record.model,
                    })}
                  />
                  <Button
                    className='h-auto min-w-0 flex-1 flex-col items-start gap-1 p-2 text-left whitespace-normal'
                    variant={
                      props.currentId === record.id ? 'secondary' : 'ghost'
                    }
                    onClick={() => props.onView(record.id)}
                  >
                    <span className='flex max-w-full flex-wrap items-center gap-2'>
                      <span className='break-all'>{record.model}</span>
                      <Badge variant='outline'>{status}</Badge>
                      {record.rating === 'good' && (
                        <Badge>{t('Good result')}</Badge>
                      )}
                      {record.rating === 'needs_review' && (
                        <Badge variant='warning'>{t('Needs review')}</Badge>
                      )}
                    </span>
                    <span className='text-muted-foreground text-xs'>
                      {record.group} ·{' '}
                      {new Intl.DateTimeFormat(locale, {
                        dateStyle: 'short',
                        timeStyle: 'short',
                      }).format(record.createdAt)}
                    </span>
                  </Button>
                </div>
              )
            })}
          </div>
        )}
        <DataTablePagination table={table} compact />
      </CardContent>
    </Card>
  )
}
