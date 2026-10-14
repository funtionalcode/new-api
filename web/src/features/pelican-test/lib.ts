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

import { PLAYGROUND_PARAMETER_CONTROLS } from '@/features/playground/lib/parameters/playground-parameters'

export const PELICAN_TEST_PROMPT =
  '创建一个HTML，内容是SVG绘制一个鹈鹕骑自行车的2D动画'

const outputTokenControl = PLAYGROUND_PARAMETER_CONTROLS.find(
  (control) => control.key === 'max_tokens'
)
if (!outputTokenControl) throw new Error('未配置输出 Token 参数范围')

export const PELICAN_DEFAULT_OUTPUT_TOKENS = 16384
export const PELICAN_MAX_OUTPUT_TOKENS = outputTokenControl.max
const outputTokenLimitSchema = z
  .number()
  .int()
  .min(1)
  .max(PELICAN_MAX_OUTPUT_TOKENS)
export const pelicanGenerationOptionsSchema = z.object({
  maxTokens: outputTokenLimitSchema,
})

export type PelicanTestRating = 'good' | 'needs_review'
const assessmentSchema = z.object({
  id: z.string(),
  model: z.string(),
  group: z.string(),
  createdAt: z.number().finite(),
  status: z.enum(['success', 'error']),
  content: z.string(),
  reasoningContent: z.string().optional(),
  errorMessage: z.string().optional(),
})

const recordSchema = z.object({
  id: z.string(),
  model: z.string(),
  group: z.string(),
  createdAt: z.number().finite(),
  status: z.enum(['running', 'success', 'error', 'cancelled']),
  content: z.string(),
  reasoningContent: z.string().optional(),
  durationMs: z.number().nonnegative().optional(),
  maxTokens: outputTokenLimitSchema.optional(),
  finishReason: z.string().optional(),
  errorMessage: z.string().optional(),
  rating: z.enum(['good', 'needs_review']).optional(),
  assessments: z.array(assessmentSchema).default([]),
})

export type PelicanTestRecord = z.infer<typeof recordSchema>
export type PelicanTestAssessment = z.infer<typeof assessmentSchema>

export function historyStorageKey(userId: number): string {
  return `pelican_test_history:v1:${userId}`
}

export function extractHtmlDocument(content: string): string {
  const fenced = content.match(/```html\s*([\s\S]*?)```/i)
  if (fenced) return fenced[1].trim()
  const document = content.match(
    /(?:<!doctype\s+html[^>]*>\s*)?<html\b[\s\S]*?<\/html>/i
  )
  if (document) return document[0].trim()
  const svg = content.match(/<svg\b[\s\S]*?<\/svg>/i)
  return svg?.[0] ?? content.replaceAll(/^```[^\n]*\n|\n```$/g, '').trim()
}

export function hasSvgMarkup(content: string): boolean {
  return /<svg\b/i.test(extractHtmlDocument(content))
}

export function readPelicanTestHistory(
  storageKey: string
): PelicanTestRecord[] {
  const raw = window.localStorage.getItem(storageKey)
  if (!raw) return []
  return z.array(recordSchema).parse(JSON.parse(raw))
}

export function writePelicanTestHistory(
  storageKey: string,
  records: PelicanTestRecord[]
): void {
  window.localStorage.setItem(storageKey, JSON.stringify(records))
}

export function buildPreviewDocument(content: string): string {
  const document = new DOMParser().parseFromString(
    extractHtmlDocument(content),
    'text/html'
  )
  document
    .querySelectorAll('base, meta[http-equiv], iframe, object, embed')
    .forEach((element) => element.remove())
  const policy = document.createElement('meta')
  policy.httpEquiv = 'Content-Security-Policy'
  policy.content =
    "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-src 'none'"
  document.head.prepend(policy)
  return `<!doctype html>${document.documentElement.outerHTML}`
}
