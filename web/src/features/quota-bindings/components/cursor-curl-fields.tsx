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

type CursorCurlFieldsProps = {
  form: QuotaBindingFormState
  onChange: (patch: Partial<QuotaBindingFormState>) => void
}

export function CursorCurlFields(props: CursorCurlFieldsProps) {
  const { t } = useTranslation()

  return (
    <>
      <div className='space-y-2'>
        <Label htmlFor='cursor-current-period-usage-curl'>
          {t('Current Period Usage Curl')}
        </Label>
        <Textarea
          id='cursor-current-period-usage-curl'
          value={props.form.request_curl}
          placeholder={
            props.form.id && props.form.has_curl
              ? t('Leave blank to keep unchanged')
              : t('Paste the Cursor current period usage curl command')
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
        <Label htmlFor='cursor-aggregated-usage-curl'>
          {t('Aggregated Usage Curl')}
        </Label>
        <Textarea
          id='cursor-aggregated-usage-curl'
          value={props.form.usage_amount_curl}
          placeholder={
            props.form.id && props.form.has_usage_amount_curl
              ? t('Leave blank to keep unchanged')
              : t('Paste the Cursor aggregated usage curl command')
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
        <Label htmlFor='cursor-plan-info-curl'>{t('Plan Info Curl')}</Label>
        <Textarea
          id='cursor-plan-info-curl'
          value={props.form.usage_cost_curl}
          placeholder={
            props.form.id && props.form.has_usage_cost_curl
              ? t('Leave blank to keep unchanged')
              : t('Paste the Cursor plan info curl command')
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
