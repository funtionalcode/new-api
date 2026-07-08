export type CliproxyAuthFileType = 'codex' | 'claude'

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

function isClaudeAuthFileName(value?: string): boolean {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
    .replaceAll('\\', '/')
  if (!normalized) {
    return false
  }
  const name = normalized.split('/').at(-1) || ''
  return name.startsWith('claude-') || name.startsWith('claude_')
}

export function getCliproxyAuthFileType(
  source: CliproxyAuthFileTypeSource
): CliproxyAuthFileType {
  if (
    isClaudePlanType(source.last_plan_type) ||
    isClaudeAuthFileName(source.auth_file) ||
    isClaudeAuthFileName(source.auth_name)
  ) {
    return 'claude'
  }
  return 'codex'
}

export function getCliproxyAuthFileTypeLabel(
  type: CliproxyAuthFileType
): string {
  return type === 'claude' ? 'Claude' : 'Codex'
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
    .replace(/^(codex|claude)[-_]/i, '')
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
