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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePagination, useDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Skeleton } from '@/components/ui/skeleton'
import { listSystemTasks } from '@/features/system-settings/api'
import type { SystemTask } from '@/features/system-settings/types'
import { createServerError } from '@/lib/server-error-message'

import { SYSTEM_TASK_TYPE_LABEL } from '../constants'
import { SystemTasksTable } from './system-tasks-table'

const EMPTY_TASKS: SystemTask[] = []

export function SystemTaskGroupDialog(props: {
  task: SystemTask
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 20 })
  const runsQuery = useQuery({
    queryKey: [
      'system-info',
      'system-tasks',
      'history',
      'runs',
      props.task.type,
      props.task.status,
      pagination,
    ],
    queryFn: async () => {
      const res = await listSystemTasks(pagination.pageSize, {
        scope: 'history',
        type: props.task.type,
        status: props.task.status,
        offset: pagination.pageIndex * pagination.pageSize,
      })
      if (!res.success || !Array.isArray(res.data)) {
        throw createServerError(res, t('We could not load system tasks.'))
      }
      return { tasks: res.data, total: res.total }
    },
    retry: false,
  })
  const tasks = runsQuery.data?.tasks ?? EMPTY_TASKS
  const { table } = useDataTable({
    data: tasks,
    columns: [],
    totalCount: runsQuery.data?.total ?? 0,
    manualPagination: true,
    pagination,
    onPaginationChange: setPagination,
    columnVisibilityStorageKey: false,
    columnSizingStorageKey: false,
    ensurePageInRange: (pageCount) => {
      if (
        runsQuery.isSuccess &&
        !runsQuery.isFetching &&
        pagination.pageIndex >= Math.max(1, pageCount)
      ) {
        setPagination((previous) => ({
          ...previous,
          pageIndex: Math.max(0, pageCount - 1),
        }))
      }
    },
  })

  return (
    <Dialog
      open
      onOpenChange={props.onOpenChange}
      title={t('Task runs')}
      description={`${t(SYSTEM_TASK_TYPE_LABEL[props.task.type] ?? props.task.type)} · ${t(props.task.status)}`}
      contentClassName='sm:max-w-6xl'
    >
      <div aria-busy={runsQuery.isFetching}>
        {runsQuery.isLoading && <Skeleton className='h-24 w-full' />}
        {runsQuery.isError && (
          <ErrorState
            title={t('We could not load system tasks.')}
            description={runsQuery.error.message}
            onRetry={() => void runsQuery.refetch()}
          />
        )}
        {!runsQuery.isLoading && !runsQuery.isError && (
          <>
            {tasks.length ? (
              <SystemTasksTable tasks={tasks} />
            ) : (
              <EmptyState
                title={t('No historical system tasks.')}
                className='min-h-32'
                bordered
              />
            )}
            <div className='mt-3'>
              <DataTablePagination table={table} compact />
            </div>
          </>
        )}
      </div>
    </Dialog>
  )
}
