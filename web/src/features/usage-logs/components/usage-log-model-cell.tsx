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
  AiBrain01Icon,
  AiBrain02Icon,
  AiBrain03Icon,
  AiBrain04Icon,
  AiBrain05Icon,
  BrainCircuitIcon,
  BrainCogIcon,
  CancelCircleIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import type { UsageLog } from '../data/schema'
import {
  formatModelName,
  getReasoningEffortVariant,
  isWebsocketLog,
  parseLogOther,
} from '../lib/format'
import { ModelBadge } from './model-badge'

const REASONING_EFFORT_ICONS = {
  none: CancelCircleIcon,
  minimal: AiBrain01Icon,
  low: AiBrain02Icon,
  medium: AiBrain03Icon,
  high: AiBrain04Icon,
  xhigh: AiBrain05Icon,
  max: BrainCircuitIcon,
} as const

export function UsageLogModelCell(props: { log: UsageLog }) {
  const { t } = useTranslation()
  const other = parseLogOther(props.log.other)
  const modelInfo = formatModelName(props.log)
  const reasoningEffort = other?.reasoning_effort?.trim()
  const reasoningEffortLabel = reasoningEffort
    ? `${t('Reasoning Effort')}: ${reasoningEffort}`
    : undefined
  const normalizedReasoningEffort = reasoningEffort?.toLowerCase()
  const reasoningEffortIcon = normalizedReasoningEffort
    ? (REASONING_EFFORT_ICONS[
        normalizedReasoningEffort as keyof typeof REASONING_EFFORT_ICONS
      ] ?? BrainCogIcon)
    : BrainCogIcon
  const showWebsocketBadge = isWebsocketLog(other)

  return (
    <div className='flex w-fit max-w-full items-center gap-1'>
      <ModelBadge
        modelName={modelInfo.name}
        actualModel={modelInfo.actualModel}
      />
      {reasoningEffort ? (
        <TooltipProvider delay={0}>
          <Tooltip>
            <TooltipTrigger
              render={
                <StatusBadge
                  variant={getReasoningEffortVariant(reasoningEffort)}
                  size='sm'
                  copyable={false}
                  showDot={false}
                  aria-label={reasoningEffortLabel}
                  role='img'
                  tabIndex={0}
                  data-reasoning-effort={reasoningEffort}
                  className='border-border/60 bg-muted/30 size-5 shrink-0 cursor-help justify-center border p-0 [&>svg]:size-3.5'
                >
                  <HugeiconsIcon
                    icon={reasoningEffortIcon}
                    strokeWidth={2}
                    aria-hidden='true'
                  />
                </StatusBadge>
              }
            />
            <TooltipContent>{reasoningEffortLabel}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      ) : null}
      {showWebsocketBadge ? (
        <StatusBadge
          label='WS'
          variant='blue'
          size='sm'
          copyable={false}
          showDot={false}
          title='WebSocket'
          aria-label='WebSocket'
          className='border-border/60 bg-muted/30 h-5 w-fit border px-1.5 font-mono text-[11px]'
        />
      ) : null}
    </div>
  )
}
