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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { parseJevDraft } from '../../../lib/structured/jev'
import { getPlaygroundGenerationMode } from '../../../lib/task/playground-task-models'
import { PlaygroundModeTabs } from '../../input/playground-mode-tabs'
import { JevPlayground } from '../jev-playground'

const props = {
  model: 'jev-latest',
  group: 'default',
  models: [{ value: 'jev-latest', label: 'jev-latest' }],
  groups: [{ value: 'default', label: 'default', ratio: 1 }],
  isModelLoading: false,
  onModelChange: vi.fn(),
  onGroupChange: vi.fn(),
}
const response = {
  model: 'jev-latest',
  answers: {
    satisfied: { type: 'noul', noul: 0 },
    sentiment: { type: 'choice', choice: 'neutral' },
    rating: { type: 'score', score: 0 },
  },
  usage: { input_tokens: 1250, output_tokens: 40 },
}

describe('Jev 结构化游乐场', () => {
  it('结构化标签可切换，窄屏允许标签换行，忙碌时禁止切换', async () => {
    const onModeChange = vi.fn()
    const view = render(
      <PlaygroundModeTabs mode='chat' onModeChange={onModeChange} />
    )
    expect(screen.getByRole('tablist')).toHaveClass(
      'flex-wrap',
      'max-w-full',
      'group-data-horizontal/tabs:h-auto'
    )
    await userEvent.click(screen.getByRole('tab', { name: 'Structured' }))
    expect(onModeChange).toHaveBeenCalledWith('structured')
    view.rerender(
      <PlaygroundModeTabs mode='chat' onModeChange={onModeChange} disabled />
    )
    expect(screen.getByRole('tab', { name: 'Structured' })).toHaveAttribute(
      'aria-disabled',
      'true'
    )
    onModeChange.mockClear()
    fireEvent.click(screen.getByRole('tab', { name: 'Structured' }))
    expect(onModeChange).not.toHaveBeenCalled()
  })
  it('选择 Jev 模型时使用结构化模式，普通聊天模型保持原模式', () => {
    expect(getPlaygroundGenerationMode('chat', 'jev-latest')).toBe('structured')
    expect(getPlaygroundGenerationMode('chat', 'typesafe/jev-1.13.0')).toBe(
      'structured'
    )
    expect(getPlaygroundGenerationMode('chat', 'gpt-4o')).toBe('chat')
    expect(getPlaygroundGenerationMode('structured', 'my-jev-alias')).toBe(
      'structured'
    )
  })

  it.each([
    ['null', '{"q":{"type":"noul","instructions":"Check"}}', 'state_schema'],
    ['{"text":', '{}', 'state_json'],
    ['{}', '{', 'questions_json'],
    ['{}', '{}', 'questions_schema'],
    [
      '{}',
      '{"q":{"type":"choice","instructions":"Check","criteria":{}}}',
      'questions_schema',
    ],
    [
      '{}',
      '{"q":{"type":"score","instructions":"Rate","criteria":["one"]}}',
      'questions_schema',
    ],
    ['{}', '{"q":{"type":"noul","instructions":true}}', 'questions_schema'],
    [
      '{}',
      '{"q":{"type":"noul","instructions":"Check","criteria":{"maybe":"yes"}}}',
      'questions_schema',
    ],
  ])('无效结构 %s / %s 返回 %s', (state, questions, error) => {
    expect(parseJevDraft(state, questions)).toEqual({ error })
  })

  it.each(['"plain text"', '[{"text":"example"}]', '{"text":"example"}'])(
    '保留状态 JSON %s 及嵌套说明',
    (state) => {
      const questions = {
        ' result ': {
          type: 'choice',
          instructions: { task: ['classify'] },
          criteria: { yes: null, no: 'negative' },
        },
      }
      expect(parseJevDraft(state, JSON.stringify(questions))).toEqual({
        data: { state: JSON.parse(state), questions },
      })
    }
  )

  it('无效 JSON 和空问题阻止发送，并保留输入供修正', async () => {
    const post = vi.spyOn(api, 'post')
    render(<JevPlayground {...props} />)
    fireEvent.input(screen.getByRole('textbox', { name: 'State (JSON)' }), {
      target: { value: '{broken' },
    })
    await userEvent.click(
      screen.getByRole('button', { name: 'Run evaluation' })
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      'State must contain valid JSON.'
    )
    expect(screen.getByRole('textbox', { name: 'State (JSON)' })).toHaveValue(
      '{broken'
    )
    expect(post).not.toHaveBeenCalled()
  })

  it('发送原生 state/questions 与所选分组，展示三个答案和用量', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({ data: response })
    render(<JevPlayground {...props} />)
    await userEvent.click(screen.getByRole('button', { name: 'Load example' }))
    const state = JSON.parse(
      (
        screen.getByRole('textbox', {
          name: 'State (JSON)',
        }) as HTMLTextAreaElement
      ).value
    )
    const questions = JSON.parse(
      (
        screen.getByRole('textbox', {
          name: 'Questions (JSON)',
        }) as HTMLTextAreaElement
      ).value
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Run evaluation' })
    )
    await waitFor(() =>
      expect(
        screen.getByRole('region', { name: 'Evaluation result' })
      ).toBeVisible()
    )
    expect(post).toHaveBeenCalledWith(
      '/pg/systemone',
      { model: 'jev-latest', group: 'default', state, questions },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    expect(screen.getByText('0.00')).toBeVisible()
    expect(screen.getByText('neutral', { selector: 'dd' })).toBeVisible()
    expect(screen.getByText('0', { selector: 'dd' })).toBeVisible()
    expect(screen.getByText(/Input Tokens: 1,250/)).toBeVisible()
  })

  it('上游错误在页面显示，保留草稿且允许重试', async () => {
    vi.spyOn(api, 'post').mockRejectedValue({
      response: { data: { error: { message: 'Insufficient quota' } } },
    })
    render(<JevPlayground {...props} />)
    await userEvent.click(screen.getByRole('button', { name: 'Load example' }))
    await userEvent.click(
      screen.getByRole('button', { name: 'Run evaluation' })
    )
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Insufficient quota'
    )
    expect(screen.getByRole('button', { name: 'Run evaluation' })).toBeEnabled()
    expect(
      (
        screen.getByRole('textbox', {
          name: 'Questions (JSON)',
        }) as HTMLTextAreaElement
      ).value
    ).toContain('sentiment')
  })

  it('停止请求后丢弃迟到响应，离开页面时取消正在执行的请求', async () => {
    let finish!: (value: { data: typeof response }) => void
    const post = vi.spyOn(api, 'post').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    const view = render(<JevPlayground {...props} />)
    await userEvent.click(screen.getByRole('button', { name: 'Load example' }))
    await userEvent.click(
      screen.getByRole('button', { name: 'Run evaluation' })
    )
    const firstSignal = post.mock.calls[0][2]?.signal
    await userEvent.click(screen.getByRole('button', { name: 'Stop' }))
    expect(firstSignal?.aborted).toBe(true)
    await act(async () => {
      finish({ data: response })
    })
    expect(
      screen.queryByRole('region', { name: 'Evaluation result' })
    ).not.toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Run evaluation' })
    )
    const secondSignal = post.mock.calls[1][2]?.signal
    view.unmount()
    expect(secondSignal?.aborted).toBe(true)
  })

  it('模型列表尚未加载或没有可用模型时禁止发送', () => {
    const view = render(<JevPlayground {...props} isModelLoading />)
    expect(
      screen.getByRole('button', { name: 'Run evaluation' })
    ).toBeDisabled()
    view.rerender(<JevPlayground {...props} model='' models={[]} />)
    expect(
      screen.getByRole('button', { name: 'Run evaluation' })
    ).toBeDisabled()
  })

  it('无效上游响应显示错误而不产生误导性答案', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({ data: { answers: {} } })
    render(<JevPlayground {...props} />)
    await userEvent.click(screen.getByRole('button', { name: 'Load example' }))
    await userEvent.click(
      screen.getByRole('button', { name: 'Run evaluation' })
    )
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Invalid structured response'
    )
    expect(
      screen.queryByRole('region', { name: 'Evaluation result' })
    ).not.toBeInTheDocument()
  })
})
