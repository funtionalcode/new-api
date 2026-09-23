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

const structuredValue = z.union([
  z.string(),
  z.record(z.string(), z.unknown()),
  z.array(z.unknown()),
])
const questionSchema = z.discriminatedUnion('type', [
  z.object({
    type: z.literal('noul'),
    instructions: structuredValue,
    criteria: z
      .object({ true: z.string().optional(), false: z.string().optional() })
      .strict()
      .optional(),
  }),
  z.object({
    type: z.literal('choice'),
    instructions: structuredValue,
    criteria: z
      .record(z.string(), z.string().nullable())
      .refine((value) => Object.keys(value).length > 0),
  }),
  z.object({
    type: z.literal('score'),
    instructions: structuredValue,
    criteria: z.array(z.string()).min(2),
  }),
])
const questionsSchema = z
  .record(
    z.string().refine((key) => key.trim().length > 0),
    questionSchema
  )
  .refine((value) => Object.keys(value).length > 0)

export interface JevRequest {
  model: string
  group?: string
  state: z.infer<typeof structuredValue>
  questions: z.infer<typeof questionsSchema>
}

export const jevResponseSchema = z
  .object({
    model: z.string().min(1),
    answers: z
      .record(z.string(), z.unknown())
      .refine((value) => Object.keys(value).length > 0),
    usage: z
      .object({
        input_tokens: z.number().int().nonnegative(),
        output_tokens: z.number().int().nonnegative(),
      })
      .passthrough(),
  })
  .passthrough()

export type JevResponse = z.infer<typeof jevResponseSchema>

export function parseJevDraft(
  stateText: string,
  questionsText: string
):
  | { data: Pick<JevRequest, 'state' | 'questions'>; error?: never }
  | {
      data?: never
      error:
        | 'state_json'
        | 'state_schema'
        | 'questions_json'
        | 'questions_schema'
    } {
  let state: unknown
  let questions: unknown
  try {
    state = JSON.parse(stateText)
  } catch {
    return { error: 'state_json' }
  }
  const parsedState = structuredValue.safeParse(state)
  if (!parsedState.success) return { error: 'state_schema' }
  try {
    questions = JSON.parse(questionsText)
  } catch {
    return { error: 'questions_json' }
  }
  const parsedQuestions = questionsSchema.safeParse(questions)
  if (!parsedQuestions.success) return { error: 'questions_schema' }
  return { data: { state: parsedState.data, questions: parsedQuestions.data } }
}
