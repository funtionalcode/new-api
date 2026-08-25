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
import {
  getCliproxyPlanLabel,
  type CliproxyAuthFileType,
} from './auth-file-type'

export type CliproxyPlanLabelConfig = {
  label: string
  multiplier?: string
  className: string
}

export function normalizeCliproxyPlanKey(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value
    .trim()
    .toLowerCase()
    .replaceAll(/[-_\s]/g, '')
}

export function getCliproxyPlanLabelConfig(
  type: CliproxyAuthFileType,
  value: unknown
): CliproxyPlanLabelConfig | null {
  const planLabel = getCliproxyPlanLabel(
    typeof value === 'string' ? value : undefined
  )
  const key = normalizeCliproxyPlanKey(planLabel)
  if (!key) return null

  if (type === 'claude') {
    switch (key) {
      case 'claudemax20x':
      case 'defaultclaudemax20x':
      case 'max20x':
      case 'pro20x':
        return {
          label: 'Max',
          multiplier: '20x',
          className:
            'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-950/35 dark:text-amber-200',
        }
      case 'claudemax5x':
      case 'defaultclaudemax5x':
      case 'max5x':
      case 'pro5x':
      case 'prolite':
        return {
          label: 'Max',
          multiplier: '5x',
          className:
            'border-sky-300 bg-sky-50 text-sky-800 dark:border-sky-700 dark:bg-sky-950/35 dark:text-sky-200',
        }
      case 'claudemax':
      case 'planmax':
        return {
          label: 'Max',
          className:
            'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-950/35 dark:text-amber-200',
        }
      case 'claudepro':
      case 'planpro':
      case 'plus':
      case 'pro':
        return {
          label: 'Pro',
          className:
            'border-indigo-300 bg-indigo-50 text-indigo-800 dark:border-indigo-700 dark:bg-indigo-950/35 dark:text-indigo-200',
        }
    }
  }

  if (type === 'codex') {
    if (key === 'pro' || key === 'pro20x') {
      return {
        label: 'Pro',
        multiplier: '20x',
        className:
          'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-950/35 dark:text-amber-200',
      }
    }
    if (key === 'prolite' || key === 'pro5x') {
      return {
        label: 'Pro',
        multiplier: '5x',
        className:
          'border-sky-300 bg-sky-50 text-sky-800 dark:border-sky-700 dark:bg-sky-950/35 dark:text-sky-200',
      }
    }
    if (key === 'plus') {
      return {
        label: 'Plus',
        className:
          'border-indigo-300 bg-indigo-50 text-indigo-800 dark:border-indigo-700 dark:bg-indigo-950/35 dark:text-indigo-200',
      }
    }
  }

  if (
    type === 'xai' &&
    [
      'supergroklite',
      'supergrok',
      'supergrokplus',
      'supergrokheavy',
      'xpremium+',
    ].includes(key)
  ) {
    return {
      label: planLabel,
      className:
        'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/35 dark:text-emerald-200',
    }
  }

  if (key === 'team' || key === 'planteam' || key === 'claudeteam') {
    return {
      label: 'Team',
      className:
        'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/35 dark:text-emerald-200',
    }
  }

  if (key === 'claudeenterprise') {
    return {
      label: 'Enterprise',
      className:
        'border-violet-300 bg-violet-50 text-violet-800 dark:border-violet-700 dark:bg-violet-950/35 dark:text-violet-200',
    }
  }

  if (key === 'free' || key === 'planfree' || key === 'claudefree') {
    return {
      label: 'Free',
      className: 'border-border bg-muted text-muted-foreground',
    }
  }

  return {
    label: planLabel,
    className: 'border-border bg-background text-muted-foreground',
  }
}
