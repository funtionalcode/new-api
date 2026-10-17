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

import { toIntlLocale } from '@/i18n/languages'
import { formatLogQuota, formatNumber } from '@/lib/format'

import { getJevMetricChange } from '../lib/jev-comparison'

export function JevComparisonValue(props: {
  value: number | null
  money?: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  if (props.value === null) {
    return <span className='text-muted-foreground'>{t('No requests')}</span>
  }
  return (
    <span className='font-mono tabular-nums'>
      {props.money
        ? formatLogQuota(props.value)
        : formatNumber(props.value, locale)}
    </span>
  )
}

export function JevDifferenceValue(props: {
  baseline: number | null
  current: number | null
  money?: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const change = getJevMetricChange(props.baseline, props.current)
  if (change.difference === null) {
    return <span className='text-muted-foreground'>{t('No requests')}</span>
  }
  const amount = props.money
    ? formatLogQuota(Math.abs(change.difference))
    : formatNumber(Math.abs(change.difference), locale)
  let sign = ''
  if (change.difference > 0) sign = '+'
  if (change.difference < 0) sign = '-'
  let percent = t('No nonzero baseline')
  if (change.percent !== null) {
    percent = `${change.percent > 0 ? '+' : ''}${formatNumber(change.percent, locale)}%`
  }
  return (
    <div className='space-y-0.5 font-mono tabular-nums'>
      <div>
        {sign}
        {amount}
      </div>
      <div className='text-muted-foreground text-xs'>{percent}</div>
    </div>
  )
}
