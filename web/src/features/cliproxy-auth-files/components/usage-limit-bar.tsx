import { useTranslation } from 'react-i18next'

import { Progress } from '@/components/ui/progress'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

export function UsageLimitBar({
  label,
  percent,
  precise = false,
}: {
  label: string
  percent: number | null
  precise?: boolean
}) {
  const { t, i18n } = useTranslation()
  const valid = percent !== null && Number.isFinite(percent)
  const normalizedPercent = valid
    ? Math.min(100, Math.max(0, precise ? percent : Math.round(percent)))
    : 0
  let color = '[&_[data-slot=progress-indicator]]:bg-emerald-500'
  if (normalizedPercent >= 90) {
    color = '[&_[data-slot=progress-indicator]]:bg-rose-500'
  } else if (normalizedPercent >= 70) {
    color = '[&_[data-slot=progress-indicator]]:bg-amber-500'
  }

  return (
    <div className='min-w-[140px] space-y-1'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='text-muted-foreground'>{label}</span>
        <span className='font-mono font-medium tabular-nums'>
          {valid
            ? `${precise ? formatNumber(normalizedPercent, toIntlLocale(i18n.resolvedLanguage || i18n.language)) : normalizedPercent}%`
            : t('Unknown')}
        </span>
      </div>
      {valid ? (
        <Progress
          aria-label={`${label} ${t('Used')}`}
          value={normalizedPercent}
          className={cn('h-1.5', color)}
        />
      ) : null}
    </div>
  )
}
