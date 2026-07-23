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
import { CalendarRange, Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { DateTimePicker } from '@/components/datetime-picker'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  TIME_GRANULARITY_OPTIONS,
  TIME_RANGE_PRESETS,
} from '@/features/dashboard/constants'
import type { TimeGranularity } from '@/lib/time'

interface TimeRangeControlsProps {
  children?: ReactNode
  endTime?: Date
  loading?: boolean
  onEndTimeChange: (date: Date | undefined) => void
  onGranularityChange: (granularity: TimeGranularity) => void
  onRangeChange: (days: number) => void
  onStartTimeChange: (date: Date | undefined) => void
  rangeLabel: string
  selectedRange: number | null
  startTime?: Date
  timeGranularity: TimeGranularity
}

export function TimeRangeControls(props: TimeRangeControlsProps) {
  const { t } = useTranslation()

  return (
    <div className='space-y-2'>
      <div className='flex flex-col gap-2 xl:flex-row xl:items-center xl:justify-between'>
        <div className='text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs'>
          <CalendarRange className='size-3.5 shrink-0' />
          <span>{t('Date Range')}:</span>
          <span className='truncate font-mono tabular-nums'>
            {props.rangeLabel}
          </span>
        </div>

        <div className='border-border/60 bg-muted/20 flex max-w-full flex-wrap items-center gap-2 rounded-md border px-2 py-1'>
          <CalendarRange className='text-muted-foreground size-4 shrink-0' />
          <DateTimePicker
            value={props.startTime}
            onChange={props.onStartTimeChange}
            placeholder={t('Select start time')}
            className='w-[280px]'
          />
          <DateTimePicker
            value={props.endTime}
            onChange={props.onEndTimeChange}
            placeholder={t('Select end time')}
            className='w-[280px]'
          />
        </div>
      </div>

      <div className='flex items-center gap-1.5 overflow-x-auto pb-1 sm:gap-2'>
        <Tabs
          value={
            props.selectedRange === null ? '' : String(props.selectedRange)
          }
          onValueChange={(value) => props.onRangeChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            {TIME_RANGE_PRESETS.map((preset) => (
              <TabsTrigger
                key={preset.days}
                value={String(preset.days)}
                className='px-2.5 text-xs'
              >
                {t(preset.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Tabs
          value={props.timeGranularity}
          onValueChange={(value) =>
            props.onGranularityChange(value as TimeGranularity)
          }
          className='shrink-0'
        >
          <TabsList>
            {TIME_GRANULARITY_OPTIONS.map((option) => (
              <TabsTrigger
                key={option.value}
                value={option.value}
                className='px-2.5 text-xs'
              >
                {t(option.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {props.children}

        {props.loading && (
          <Loader2 className='text-muted-foreground size-4 animate-spin' />
        )}
      </div>
    </div>
  )
}
