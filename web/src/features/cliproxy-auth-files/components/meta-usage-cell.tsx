import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'

import { parseMetaQuota } from '../lib/meta-quota'
import type { CliproxyAuthFileBinding } from '../types'
import { UsageLimitBar } from './usage-limit-bar'

export function MetaUsageCell({
  binding,
}: {
  binding: Pick<CliproxyAuthFileBinding, 'last_error' | 'last_meta_quota'>
}) {
  const { t, i18n } = useTranslation()
  const quota = parseMetaQuota(binding.last_meta_quota)
  const error = binding.last_error.trim()
  if (error || quota === null) {
    return (
      <Alert variant='destructive' className='max-w-[460px]'>
        <AlertTitle>{t('Error')}</AlertTitle>
        <AlertDescription className='text-xs break-words whitespace-pre-wrap'>
          {t(error || 'Invalid Meta quota response.')}
        </AlertDescription>
      </Alert>
    )
  }
  if (!quota) {
    return (
      <span className='text-muted-foreground text-sm'>
        {t('No quota data')}
      </span>
    )
  }
  const windows = [
    {
      key: 'window',
      label:
        quota.window.durationMinutes === null
          ? t('Usage Window')
          : t('{{minutes}}-Minute Window', {
              minutes: formatNumber(
                quota.window.durationMinutes,
                toIntlLocale(i18n.resolvedLanguage || i18n.language)
              ),
            }),
      quota: quota.window,
    },
    { key: 'weekly', label: t('Weekly Window'), quota: quota.weekly },
  ]

  return (
    <div className='max-w-[460px] space-y-2'>
      <div className='grid gap-2 sm:grid-cols-2'>
        {windows.map((window) => (
          <div key={window.key} className='space-y-1'>
            <UsageLimitBar
              label={window.label}
              percent={window.quota.usedPercent}
              precise
            />
            {window.quota.resetAt !== null ? (
              <div className='text-muted-foreground text-[11px]'>
                {t('Reset')}: {formatTimestampToDate(window.quota.resetAt)}
              </div>
            ) : null}
          </div>
        ))}
      </div>
      {quota.subscriptionActive === false ? (
        <Badge variant='outline'>{t('Subscription inactive')}</Badge>
      ) : null}
    </div>
  )
}
