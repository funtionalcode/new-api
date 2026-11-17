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
import type { ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

export function CodexCreditsDetails(props: {
  credits?: string
}): ReactElement | null {
  const { t, i18n } = useTranslation()
  if (!props.credits) return null

  let credits: Record<string, unknown>
  try {
    const parsed: unknown = JSON.parse(props.credits)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null
    }
    credits = parsed as Record<string, unknown>
  } catch {
    return null
  }

  let balance: number | null = null
  if (
    typeof credits.balance === 'number' ||
    typeof credits.balance === 'string'
  ) {
    const raw = String(credits.balance).trim()
    const value = Number(raw)
    if (raw !== '' && Number.isFinite(value) && value >= 0) {
      balance = value
    }
  }
  const hasCredits = credits.has_credits
  const unlimited = hasCredits === true && credits.unlimited === true
  if (balance === null && typeof hasCredits !== 'boolean') return null

  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  let balanceLabel = t('Unknown')
  if (unlimited) {
    balanceLabel = t('Unlimited')
  } else if (balance !== null) {
    balanceLabel =
      balance > 0 && balance < 1
        ? '<1'
        : formatNumber(Math.floor(balance), locale)
  }
  let statusLabel = t('Unknown')
  if (hasCredits === false || (balance === 0 && !unlimited)) {
    statusLabel = t('No credits available')
  } else if (hasCredits === true) {
    statusLabel = t('Available')
  }

  return (
    <section
      aria-label={t('Codex credits')}
      className='border-background/15 border-t pt-2'
    >
      <dl className='grid grid-cols-[minmax(8rem,1fr)_auto] gap-x-4 gap-y-1'>
        <dt>{t('Credits balance')}</dt>
        <dd className='text-right font-mono font-semibold'>{balanceLabel}</dd>
        <dt>{t('Status')}</dt>
        <dd className='text-right'>{statusLabel}</dd>
      </dl>
    </section>
  )
}
