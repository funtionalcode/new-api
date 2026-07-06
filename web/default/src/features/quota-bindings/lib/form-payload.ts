import type {
  QuotaBindingFormData,
  QuotaBindingSavePayload,
} from '../types'

export type QuotaBindingFormState = QuotaBindingFormData & {
  has_curl?: boolean
  request_curl_touched?: boolean
  proxy_touched?: boolean
}

export function buildQuotaBindingSavePayload(
  form: QuotaBindingFormState
): QuotaBindingSavePayload {
  const isEdit = Boolean(form.id)
  const payload: QuotaBindingSavePayload = {
    id: form.id,
    name: form.name.trim(),
    note: form.note.trim(),
    enabled: form.enabled,
    plan_type: form.plan_type,
    five_hour_limit_tokens: form.five_hour_limit_tokens,
    weekly_limit_tokens: form.weekly_limit_tokens,
  }

  if (!isEdit || form.request_curl_touched) {
    const curl = form.request_curl.trim()
    if (curl || !isEdit) {
      payload.request_curl = curl
    }
  }

  if (!isEdit || form.proxy_touched) {
    payload.proxy = form.proxy.trim()
  }

  return payload
}
