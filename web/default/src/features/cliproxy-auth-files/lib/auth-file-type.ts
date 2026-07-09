export type CliproxyAuthFileType = 'codex' | 'claude' | 'xai'

const claudePlanTypes = new Set([
  'claude',
  'planmax',
  'claudemax',
  'planpro',
  'claudepro',
  'planteam',
  'claudeteam',
  'planfree',
  'claudefree',
])

const xaiPlanTypes = new Set(['xai'])

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

function isClaudePlanType(value?: string): boolean {
  return claudePlanTypes.has(normalizeCliproxyPlan(value))
}

function isXAIPlanType(value?: string): boolean {
  return xaiPlanTypes.has(normalizeCliproxyPlan(value))
}

function hasAuthFileNamePrefix(value: string | undefined, prefix: string): boolean {
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
  const normalized = String(value || '').trim().replaceAll('\\', '/')
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
