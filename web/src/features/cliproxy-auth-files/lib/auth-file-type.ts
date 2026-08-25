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
export type CliproxyAuthFileType = 'codex' | 'claude' | 'xai'

const claudePlanTypes = new Set([
  'claude',
  'planmax',
  'claudemax',
  'claudemax5x',
  'claudemax20x',
  'defaultclaudemax5x',
  'defaultclaudemax20x',
  'planpro',
  'claudepro',
  'planteam',
  'claudeteam',
  'planfree',
  'claudefree',
  'claudeenterprise',
])

const xaiPlanTypes = new Set([
  'xai',
  'xaifree',
  'supergroklite',
  'supergrok',
  'supergrokplus',
  'supergrokheavy',
  'xpremium+',
  'xpremiumplus',
  'subscriptiontiersupergroklite',
  'subscriptiontiersupergrok',
  'subscriptiontiersupergrokplus',
  'subscriptiontiersupergrokheavy',
])

interface CliproxyAuthFileTypeSource {
  auth_name?: string
  auth_file?: string
  last_plan_type?: string
}

const emailPlanSuffixPattern =
  /[-_](pro|prolite|plus|free|team|plan[-_]?max|plan[-_]?pro|plan[-_]?team|plan[-_]?free|claude[-_]?max|claude[-_]?pro|claude[-_]?team|claude[-_]?free|\d+x)$/i
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

function normalizeCliproxyPlan(value?: string): string {
  return String(value || '')
    .toLowerCase()
    .replaceAll('-', '')
    .replaceAll('_', '')
    .replaceAll(' ', '')
}

export function getCliproxyPlanLabel(value?: string): string {
  const label = String(value || '').trim()
  switch (normalizeCliproxyPlan(label)) {
    case 'subscriptiontiersupergroklite':
    case 'supergroklite':
      return 'SuperGrok Lite'
    case 'subscriptiontiersupergrok':
    case 'supergrok':
      return 'SuperGrok'
    case 'subscriptiontiersupergrokplus':
    case 'supergrokplus':
      return 'SuperGrok Plus'
    case 'subscriptiontiersupergrokheavy':
    case 'supergrokheavy':
      return 'SuperGrok Heavy'
    case 'xpremium+':
    case 'xpremiumplus':
      return 'X Premium+'
    default:
      return label
  }
}

function isClaudePlanType(value?: string): boolean {
  return claudePlanTypes.has(normalizeCliproxyPlan(value))
}

function isXAIPlanType(value?: string): boolean {
  return xaiPlanTypes.has(normalizeCliproxyPlan(value))
}

function hasAuthFileNamePrefix(
  value: string | undefined,
  prefix: string
): boolean {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
    .replaceAll('\\', '/')
  if (!normalized) {
    return false
  }
  const name = normalized.split('/').at(-1) || ''
  return name.startsWith(`${prefix}-`) || name.startsWith(`${prefix}_`)
}

export function getCliproxyAuthFileType(
  source: CliproxyAuthFileTypeSource
): CliproxyAuthFileType {
  if (
    isXAIPlanType(source.last_plan_type) ||
    hasAuthFileNamePrefix(source.auth_file, 'xai') ||
    hasAuthFileNamePrefix(source.auth_name, 'xai')
  ) {
    return 'xai'
  }
  if (
    isClaudePlanType(source.last_plan_type) ||
    hasAuthFileNamePrefix(source.auth_file, 'claude') ||
    hasAuthFileNamePrefix(source.auth_name, 'claude')
  ) {
    return 'claude'
  }
  return 'codex'
}

export function getCliproxyAuthFileTypeLabel(
  type: CliproxyAuthFileType
): string {
  if (type === 'claude') return 'Claude'
  if (type === 'xai') return 'xAI'
  return 'Codex'
}

function getAuthFileBaseName(value?: string): string {
  const normalized = String(value || '')
    .trim()
    .replaceAll('\\', '/')
  if (!normalized) {
    return ''
  }
  return normalized.split('/').at(-1) || ''
}

function emailFromAuthFileName(value?: string): string {
  const name = getAuthFileBaseName(value)
    .replace(/\.json$/i, '')
    .replace(/^(codex|claude|xai)[-_]/i, '')
    .replace(emailPlanSuffixPattern, '')
  return emailPattern.test(name) ? name : ''
}

export function getCliproxyAuthFileEmail(
  source: CliproxyAuthFileTypeSource
): string {
  return (
    emailFromAuthFileName(source.auth_name) ||
    emailFromAuthFileName(source.auth_file)
  )
}
