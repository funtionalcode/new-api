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

import { cn } from '@/lib/utils'

import type { UsageLog } from '../data/schema'
import {
  getLogCacheTokenCounts,
  getLogContextSize,
  parseLogOther,
} from '../lib/format'

interface UsageLogTokensProps {
  log: UsageLog
  variant?: 'table' | 'mobile'
}

export function UsageLogTokens(props: UsageLogTokensProps) {
  const { t } = useTranslation()
  const other = parseLogOther(props.log.other)
  const inputTokens = getLogContextSize(props.log, other)
  const completionTokens = props.log.completion_tokens || 0
  const { cacheReadTokens, cacheWriteTokens } = getLogCacheTokenCounts(other)

  if (inputTokens === 0 && completionTokens === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  const showCache = cacheReadTokens > 0 || cacheWriteTokens > 0
  const isMobile = props.variant === 'mobile'

  return (
    <div className='flex flex-col gap-0.5'>
      <span className='font-mono text-xs font-medium tabular-nums'>
        {inputTokens.toLocaleString()} / {completionTokens.toLocaleString()}
      </span>
      {showCache ? (
        <div
          className={cn(
            'flex items-center text-[11px]',
            isMobile
              ? 'text-muted-foreground flex-wrap gap-x-1.5 gap-y-0.5 leading-none'
              : 'gap-1'
          )}
        >
          {cacheReadTokens > 0 && (
            <span className={cn(!isMobile && 'text-muted-foreground/60')}>
              {t('Cache')}↓ {cacheReadTokens.toLocaleString()}
            </span>
          )}
          {cacheWriteTokens > 0 && (
            <span className={cn(!isMobile && 'text-muted-foreground/60')}>
              ↑ {cacheWriteTokens.toLocaleString()}
            </span>
          )}
        </div>
      ) : (
        isMobile && (
          <span className='text-muted-foreground/50 text-[11px] leading-none'>
            —
          </span>
        )
      )}
    </div>
  )
}
