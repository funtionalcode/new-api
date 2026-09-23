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
import { z } from 'zod'

const channelTypes = new Set([1, 48, 57, 58, 61, 62])
export function supportsAdaptiveReasoning(channelType: number): boolean {
  return channelTypes.has(channelType)
}

export const adaptiveReasoningSchema = z
  .object({
    enabled: z.boolean(),
    channel_id: z.number().int().min(0),
    model: z.string().trim().max(200),
    efforts: z.array(
      z.enum(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])
    ),
    max_reuse_generations: z
      .number()
      .refine((value) => [1, 2, 5, 10].includes(value), 'Invalid value'),
    timeout_ms: z.number().int().min(1).max(30000),
    max_chars: z.number().int().min(1).max(60000),
  })
  .superRefine((value, ctx) => {
    if (!value.enabled) return
    if (value.channel_id <= 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['channel_id'],
        message: 'Select a TypeSafe channel',
      })
    }
    if (value.efforts.length === 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['efforts'],
        message: 'Select at least one reasoning effort',
      })
    }
  })

export type AdaptiveReasoningConfig = z.infer<typeof adaptiveReasoningSchema>
export const ADAPTIVE_REASONING_DEFAULTS: AdaptiveReasoningConfig = {
  enabled: false,
  channel_id: 0,
  model: 'jev-latest',
  efforts: ['low', 'medium', 'high', 'xhigh'],
  max_reuse_generations: 10,
  timeout_ms: 1500,
  max_chars: 12000,
}

export function readAdaptiveReasoning(value: unknown): AdaptiveReasoningConfig {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return { ...ADAPTIVE_REASONING_DEFAULTS }
  }
  const config = {
    ...ADAPTIVE_REASONING_DEFAULTS,
    ...value,
  } as AdaptiveReasoningConfig
  return {
    ...config,
    model: config.model || ADAPTIVE_REASONING_DEFAULTS.model,
    efforts: config.efforts?.length
      ? config.efforts
      : [...ADAPTIVE_REASONING_DEFAULTS.efforts],
    max_reuse_generations: config.max_reuse_generations || 10,
    timeout_ms: config.timeout_ms || 1500,
    max_chars: config.max_chars || 12000,
  }
}
