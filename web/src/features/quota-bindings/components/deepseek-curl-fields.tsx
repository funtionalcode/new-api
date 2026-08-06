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

import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import type { QuotaBindingFormState } from '../lib/form-payload'

type DeepSeekCurlFieldsProps = {
  form: QuotaBindingFormState
  onChange: (patch: Partial<QuotaBindingFormState>) => void
}

export function DeepSeekCurlFields(props: DeepSeekCurlFieldsProps) {
  const { t } = useTranslation()

  return (
    <>
      <div className='space-y-2'>
        <Label htmlFor='deepseek-account-summary-curl'>
          {t('Account Summary Curl')}
        </Label>
        <Textarea
          id='deepseek-account-summary-curl'
          value={props.form.request_curl}
          placeholder={
            props.form.id && props.form.has_curl
              ? t('Leave blank to keep unchanged')
              : t('Paste the DeepSeek account summary curl command')
          }
          onChange={(event) =>
            props.onChange({
              request_curl: event.target.value,
              request_curl_touched: true,
            })
          }
        />
      </div>

      <div className='space-y-2'>
        <Label htmlFor='deepseek-usage-amount-curl'>
          {t('Usage Amount Curl')}
        </Label>
        <Textarea
          id='deepseek-usage-amount-curl'
          value={props.form.usage_amount_curl}
          placeholder={
            props.form.id && props.form.has_usage_amount_curl
              ? t('Leave blank to keep unchanged')
              : t('Paste the DeepSeek usage amount curl command')
          }
          onChange={(event) =>
            props.onChange({
              usage_amount_curl: event.target.value,
              usage_amount_curl_touched: true,
            })
          }
        />
      </div>

      <div className='space-y-2'>
        <Label htmlFor='deepseek-usage-cost-curl'>{t('Usage Cost Curl')}</Label>
        <Textarea
          id='deepseek-usage-cost-curl'
          value={props.form.usage_cost_curl}
          placeholder={
            props.form.id && props.form.has_usage_cost_curl
              ? t('Leave blank to keep unchanged')
              : t('Paste the DeepSeek usage cost curl command')
          }
          onChange={(event) =>
            props.onChange({
              usage_cost_curl: event.target.value,
              usage_cost_curl_touched: true,
            })
          }
        />
      </div>
    </>
  )
}
