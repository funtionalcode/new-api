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
import type { LogOtherData } from '../types'

type AuditIdentityValue =
  | string
  | number
  | boolean
  | string[]
  | null
  | undefined

export function formatAuditIdentity(
  username: AuditIdentityValue,
  id: AuditIdentityValue
): string | null {
  const name =
    username != null && String(username).trim() !== ''
      ? String(username).trim()
      : ''
  const idText = id != null && String(id).trim() !== '' ? String(id).trim() : ''
  if (!name && !idText) return null
  if (name && idText) return `${name} (ID: ${idText})`
  if (name) return name
  return `ID: ${idText}`
}

export function getManageTargetUserLabel(
  other: LogOtherData | null | undefined
): string | null {
  const params = other?.op?.params
  if (!params) return null

  const isUserAction = String(other.op?.action || '').startsWith('user.')
  const targetUsername =
    params.target_username ?? (isUserAction ? params.username : undefined)
  const targetId =
    params.target_user_id ?? (isUserAction ? params.id : undefined)
  return formatAuditIdentity(targetUsername, targetId)
}
