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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { generatePelicanHtml } from '../api'
import { PelicanTest } from '../index'
import {
  historyStorageKey,
  PELICAN_TEST_PROMPT,
  readPelicanTestHistory,
  writePelicanTestHistory,
  type PelicanTestRecord,
} from '../lib'

const html =
  '<!doctype html><html><body><svg viewBox="0 0 200 100"><circle r="10"/><animate attributeName="cx" values="0;100" dur="2s" repeatCount="indefinite"/></svg></body></html>'
const canvasResponse =
  '现在开始创建 HTML 动画：http://googleusercontent.com/immersive_entry_chip/0'
const canvasGuidance =
  'This response contains only a Canvas entry link, not HTML source. Test again with the source-only instructions; if it persists, use another model or an upstream that returns source.'

function savedRecord(id: string, model = 'model-a'): PelicanTestRecord {
  return {
    id,
    model,
    group: 'default',
    createdAt: 1720000000000,
    status: 'success',
    content: html,
    assessments: [],
  }
}

function renderWorkspace() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PelicanTest />
    </QueryClientProvider>
  )
}

function completion(content: string, finishReason = 'stop', reasoning = '') {
  return {
    data: {
      choices: [
        {
          message: { content, reasoning_content: reasoning },
          finish_reason: finishReason,
        },
      ],
    },
    headers: {},
  }
}

// 在网络边界模拟真实 SSE，保持请求解析、流式控制器和页面更新走生产代码。
class TestXMLHttpRequest extends EventTarget {
  static HEADERS_RECEIVED = 2
  static instances: TestXMLHttpRequest[] = []
  static async respond(_url: string, _payload: unknown, _options: unknown) {
    return completion(html)
  }
  readyState = 0
  status = 200
  responseText = ''
  response = ''
  url = ''
  headers: Record<string, string> = {}

  constructor() {
    super()
    TestXMLHttpRequest.instances.push(this)
  }

  open(_method: string, url: string) {
    this.url = url
  }
  setRequestHeader(name: string, value: string) {
    this.headers[name] = value
  }
  getAllResponseHeaders() {
    return 'content-type: text/event-stream'
  }
  send(payload: string) {
    this.readyState = TestXMLHttpRequest.HEADERS_RECEIVED
    this.dispatchEvent(new Event('readystatechange'))
    void TestXMLHttpRequest.respond(this.url, JSON.parse(payload), {
      headers: this.headers,
    }).then(
      (result) => {
        const choice = result.data.choices[0]
        this.append({ choices: [{ delta: choice.message }] })
        this.append({
          choices: [{ delta: {}, finish_reason: choice.finish_reason }],
        })
        this.append('[DONE]')
        this.finish()
      },
      (error: unknown) => {
        this.status = 400
        const body = error as { response?: { data?: unknown } }
        this.response = JSON.stringify(
          body.response?.data ?? { error: { message: 'Request failed' } }
        )
        const event = new Event('error')
        Object.defineProperty(event, 'currentTarget', { value: this })
        this.dispatchEvent(event)
      }
    )
  }
  abort() {
    this.dispatchEvent(new Event('abort'))
  }
  append(data: unknown) {
    this.responseText += `data: ${typeof data === 'string' ? data : JSON.stringify(data)}\n\n`
    this.dispatchEvent(new Event('progress'))
  }
  finish() {
    this.dispatchEvent(new Event('load'))
  }
}

beforeEach(() => {
  TestXMLHttpRequest.instances = []
  vi.stubGlobal('XMLHttpRequest', TestXMLHttpRequest)
  useAuthStore.getState().auth.setBundle({
    access_token: 'pelican-test-only-token',
    token_type: 'Bearer',
    access_expires_at: Math.floor(Date.now() / 1000) + 3600,
    user: { id: 17, username: 'alice', role: 1 },
    session: {
      sid: 'test-session',
      current: true,
      login_method: 'test',
      ip: '',
      user_agent: '',
      created_at: 0,
      last_active_at: 0,
      expires_at: Math.floor(Date.now() / 1000) + 3600,
    },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: String(url).endsWith('/groups')
        ? {
            default: { desc: 'Default', ratio: 1 },
            vip: { desc: 'VIP', ratio: 1 },
          }
        : ['model-a', 'model-b', 'grok-imagine-video-1.5'],
    },
  }))
})

it('流未结束时实时显示分段思考和输出，停止后历史保留已收到的内容', async () => {
  vi.spyOn(TestXMLHttpRequest, 'respond').mockImplementation(
    () => new Promise(() => {})
  )
  renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  await userEvent.click(submit)
  await waitFor(() => expect(TestXMLHttpRequest.instances).toHaveLength(1))
  const xhr = TestXMLHttpRequest.instances[0]
  act(() =>
    xhr.append({ choices: [{ delta: { reasoning_content: '先画车轮。' } }] })
  )
  await waitFor(() =>
    expect(
      screen.getByRole('region', { name: 'Model response' })
    ).toHaveTextContent('先画车轮。')
  )
  expect(screen.getByRole('button', { name: 'Stop' })).toBeEnabled()
  act(() => xhr.append({ choices: [{ delta: { content: '<html><body>' } }] }))
  expect(await screen.findByLabelText('Output content')).toHaveTextContent(
    '<html><body>'
  )
  act(() => xhr.append({ choices: [{ delta: { content: '<svg>' } }] }))
  await waitFor(() =>
    expect(screen.getByLabelText('Output content').textContent).toBe(
      '<html><body><svg>'
    )
  )
  await userEvent.click(screen.getByRole('button', { name: 'Stop' }))
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'cancelled',
    content: '<html><body><svg>',
    reasoningContent: '先画车轮。',
  })
  act(() => {
    xhr.append({ choices: [{ delta: { content: '晚到内容' } }] })
    xhr.append('[DONE]')
  })
  expect(readPelicanTestHistory(historyStorageKey(17))[0].content).toBe(
    '<html><body><svg>'
  )
})

it('流以 stop 结束但只返回说明文字时标记失败并保留输出，不提供空动画预览', async () => {
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(
    completion('我来创建一个 HTML 页面。', 'stop', '正在设计动画。')
  )
  renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  await userEvent.click(submit)
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'The model did not return a complete HTML or SVG document'
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'error',
    content: '我来创建一个 HTML 页面。',
    reasoningContent: '正在设计动画。',
  })
  expect(screen.queryByRole('tab', { name: 'Preview' })).not.toBeInTheDocument()
})

it.each(['gemini-web-4-argon', 'model-a'])(
  '生成模型 %s 使用统一源码输出约束，原始提示词和流式参数保持不变',
  async (model) => {
    const post = vi
      .spyOn(TestXMLHttpRequest, 'respond')
      .mockResolvedValue(completion(html))
    await generatePelicanHtml(
      model,
      'default',
      16384,
      new AbortController().signal
    )
    expect(post).toHaveBeenCalledExactlyOnceWith(
      '/pg/chat/completions',
      {
        model,
        group: 'default',
        messages: [
          {
            role: 'system',
            content: expect.stringMatching(
              /<!DOCTYPE html>[\s\S]*<\/html>[\s\S]*Canvas[\s\S]*immersive_entry_chip/
            ),
          },
          { role: 'user', content: PELICAN_TEST_PROMPT },
        ],
        stream: true,
        max_tokens: 16384,
      },
      expect.any(Object)
    )
  }
)

it('只返回 Canvas 入口链接时明确原因，保留输出和思考且不自动重试', async () => {
  const post = vi
    .spyOn(TestXMLHttpRequest, 'respond')
    .mockResolvedValue(completion(canvasResponse, 'stop', '正在设计动画。'))
  renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  await userEvent.click(submit)
  const alert = await screen.findByRole('alert')
  expect(alert).toHaveTextContent(
    'The model returned a Canvas link instead of HTML source'
  )
  expect(alert).toHaveTextContent(canvasGuidance)
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'error',
    content: canvasResponse,
    reasoningContent: '正在设计动画。',
    finishReason: 'stop',
  })
  expect(screen.getByLabelText('Output content')).toHaveTextContent(
    canvasResponse
  )
  expect(screen.queryByRole('tab', { name: 'Preview' })).not.toBeInTheDocument()
  expect(post).toHaveBeenCalledTimes(1)
})

it('旧的 Canvas 链接失败历史也展示原因说明，不覆盖原记录或发起请求', async () => {
  const record: PelicanTestRecord = {
    ...savedRecord('canvas-old', 'gemini-web-4-argon'),
    status: 'error',
    content: canvasResponse,
    errorMessage: 'The model did not return a complete HTML or SVG document',
  }
  writePelicanTestHistory(historyStorageKey(17), [record])
  const stored = localStorage.getItem(historyStorageKey(17))
  const post = vi.spyOn(TestXMLHttpRequest, 'respond')
  renderWorkspace()
  expect(await screen.findByRole('alert')).toHaveTextContent(canvasGuidance)
  expect(localStorage.getItem(historyStorageKey(17))).toBe(stored)
  expect(post).not.toHaveBeenCalled()
})

it('手动重测 Canvas 链接失败后保存新源码结果，保留旧失败历史', async () => {
  const record: PelicanTestRecord = {
    ...savedRecord('canvas-old'),
    status: 'error',
    content: canvasResponse,
    errorMessage: 'The model did not return a complete HTML or SVG document',
  }
  writePelicanTestHistory(historyStorageKey(17), [record])
  const post = vi
    .spyOn(TestXMLHttpRequest, 'respond')
    .mockResolvedValue(completion(html))
  renderWorkspace()
  const rerun = await screen.findByRole('button', {
    name: 'Test again with selected model',
  })
  await waitFor(() => expect(rerun).toBeEnabled())
  await userEvent.click(rerun)
  await screen.findByText('SVG detected')
  expect(readPelicanTestHistory(historyStorageKey(17))).toEqual([
    expect.objectContaining({ status: 'success', content: html }),
    record,
  ])
  expect(post).toHaveBeenCalledTimes(1)
})

it('完整源码内包含 Canvas 链接时仍可预览，不误报为仅返回链接', async () => {
  const content = html.replace(
    '<body>',
    '<body><!-- http://googleusercontent.com/immersive_entry_chip/0 -->'
  )
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(completion(content))
  renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  await userEvent.click(submit)
  await screen.findByText('SVG detected')
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  expect(screen.getByRole('tab', { name: 'Preview' })).toBeEnabled()
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'success',
    content,
  })
})

it.each(['eof', 'upstream-error'] as const)(
  '流因 %s 中断时保留最后一段输出和思考，提示原因且不自动重试',
  async (failure) => {
    const transport = vi
      .spyOn(TestXMLHttpRequest, 'respond')
      .mockImplementation(() => new Promise(() => {}))
    renderWorkspace()
    const submit = screen.getByRole('button', {
      name: 'Generate and save result',
    })
    await waitFor(() => expect(submit).toBeEnabled())
    await userEvent.click(submit)
    await waitFor(() => expect(TestXMLHttpRequest.instances).toHaveLength(1))
    act(() => {
      const xhr = TestXMLHttpRequest.instances[0]
      xhr.append({
        choices: [
          {
            delta: {
              reasoning_content: '检查车架。',
              content: '<html><svg>最后一段',
            },
          },
        ],
      })
      if (failure === 'eof') xhr.finish()
      else xhr.append({ error: { message: 'Upstream stopped generating' } })
    })
    expect(await screen.findByRole('alert')).toHaveTextContent(
      failure === 'eof' ? 'Connection closed' : 'Upstream stopped generating'
    )
    expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
      status: 'error',
      content: '<html><svg>最后一段',
      reasoningContent: '检查车架。',
    })
    expect(transport).toHaveBeenCalledTimes(1)
    expect(submit).toBeEnabled()
  }
)

it('think 标签跨分段返回时实时分离思考，正文内同名字符串保持不变', async () => {
  vi.spyOn(TestXMLHttpRequest, 'respond').mockImplementation(
    () => new Promise(() => {})
  )
  renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  await userEvent.click(submit)
  await waitFor(() => expect(TestXMLHttpRequest.instances).toHaveLength(1))
  const xhr = TestXMLHttpRequest.instances[0]
  act(() => {
    xhr.append({ choices: [{ delta: { content: '<thi' } }] })
    xhr.append({ choices: [{ delta: { content: 'nk>先画车架。' } }] })
  })
  await waitFor(() =>
    expect(
      screen.getByRole('region', { name: 'Model response' })
    ).toHaveTextContent('先画车架。')
  )
  expect(screen.queryByLabelText('Output content')).not.toBeInTheDocument()
  const document =
    '<html><body><svg></svg><script>const text="<think>正文内容</think>";</script></body></html>'
  act(() => {
    xhr.append({
      choices: [{ delta: { content: `</think>${document.slice(0, -7)}` } }],
    })
    xhr.append({ choices: [{ delta: {}, finish_reason: 'stop' }] })
    // finish_reason 不是停止读取的信号；用 DONE 确认整条流结束。
    xhr.append({ choices: [{ delta: { content: document.slice(-7) } }] })
    xhr.append('[DONE]')
  })
  await screen.findByText('SVG detected')
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'success',
    content: document,
    reasoningContent: '先画车架。',
    finishReason: 'stop',
  })
})

it('评审流在完成前默认展开显示实时思考与内容，停止后保留部分评审且不评审下一个结果', async () => {
  writePelicanTestHistory(historyStorageKey(17), [
    savedRecord('a'),
    savedRecord('b'),
  ])
  const transport = vi
    .spyOn(TestXMLHttpRequest, 'respond')
    .mockImplementation(() => new Promise(() => {}))
  renderWorkspace()
  await screen.findAllByRole('checkbox')
  for (const checkbox of screen.getAllByRole('checkbox')) {
    await userEvent.click(checkbox)
  }
  const submit = screen.getByRole('button', {
    name: 'Assess selected history (2)',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  await userEvent.click(submit)
  await waitFor(() => expect(TestXMLHttpRequest.instances).toHaveLength(1))
  act(() =>
    TestXMLHttpRequest.instances[0].append({
      choices: [
        {
          delta: {
            reasoning_content: '检查动画循环。',
            content: '车轮结构完整，',
          },
        },
      ],
    })
  )
  const assessments = await screen.findAllByRole('region', {
    name: 'Assessment history',
  })
  await waitFor(() =>
    expect(
      within(assessments[0]).getByLabelText('Output content')
    ).toBeVisible()
  )
  expect(
    within(assessments[0]).getByLabelText('Output content')
  ).toHaveTextContent('车轮结构完整，')
  await userEvent.click(screen.getByRole('button', { name: 'Stop' }))
  await waitFor(() =>
    expect(
      readPelicanTestHistory(historyStorageKey(17))[0].assessments[0].status
    ).toBe('cancelled')
  )
  expect(within(assessments[0]).getByLabelText('Output content')).toBeVisible()
  expect(
    readPelicanTestHistory(historyStorageKey(17))[0].assessments[0]
  ).toMatchObject({
    content: '车轮结构完整，',
    reasoningContent: '检查动画循环。',
  })
  expect(
    readPelicanTestHistory(historyStorageKey(17))[1].assessments
  ).toHaveLength(0)
  expect(transport).toHaveBeenCalledTimes(1)
})

it('生成时离开页面会保存尚未落盘的分段内容，重新打开后恢复为中断历史', async () => {
  vi.spyOn(TestXMLHttpRequest, 'respond').mockImplementation(
    () => new Promise(() => {})
  )
  const workspace = renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  await userEvent.click(submit)
  await waitFor(() => expect(TestXMLHttpRequest.instances).toHaveLength(1))
  act(() =>
    TestXMLHttpRequest.instances[0].append({
      choices: [{ delta: { content: '<html><svg>待完成' } }],
    })
  )
  workspace.unmount()
  expect(readPelicanTestHistory(historyStorageKey(17))[0].content).toBe(
    '<html><svg>待完成'
  )
  renderWorkspace()
  expect(await screen.findByLabelText('Output content')).toHaveTextContent(
    '<html><svg>待完成'
  )
  expect(
    screen.getByRole('region', { name: 'Result for model-a' })
  ).toHaveTextContent('Generation was interrupted')
})

it('生成响应的思考和完整输出默认可见，并且重新打开历史后仍分别保留', async () => {
  const reasoning = '先用 SVG 绘制车架和鹈鹕，再给车轮添加循环动画。'
  const content = `实现说明\n\n\`\`\`html\n${html}\n\`\`\``
  const user = userEvent.setup()
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(
    completion(content, 'stop', reasoning)
  )
  const workspace = renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(await screen.findByText(reasoning)).toBeVisible()
  expect(screen.getByRole('tab', { name: 'Output' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  expect(screen.getByLabelText('Output content').textContent).toBe(content)
  expect(
    screen.getByRole('button', { name: 'Copy reasoning content' })
  ).toBeEnabled()
  expect(
    screen.getByRole('button', { name: 'Copy output content' })
  ).toBeEnabled()
  await user.click(
    screen.getByRole('button', { name: 'Copy reasoning content' })
  )
  expect(await navigator.clipboard.readText()).toBe(reasoning)
  await user.click(screen.getByRole('button', { name: 'Copy output content' }))
  expect(await navigator.clipboard.readText()).toBe(content)
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    reasoningContent: reasoning,
    content,
    status: 'success',
  })
  workspace.unmount()
  renderWorkspace()
  expect(await screen.findByText(reasoning)).toBeVisible()
  expect(screen.getByLabelText('Output content').textContent).toBe(content)
  await userEvent.click(
    screen.getByRole('button', { name: 'Reasoning process' })
  )
  expect(
    screen.getByRole('button', { name: 'Reasoning process' })
  ).toHaveAttribute('aria-expanded', 'false')
})

it('响应以 think 标签返回思考时，将思考与最终 HTML 分开保存', async () => {
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(
    completion(`<think>先画车轮。</think>\n${html}`)
  )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(await screen.findByText('先画车轮。')).toBeVisible()
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    reasoningContent: '先画车轮。',
    content: html,
  })
})

it('模型只返回思考而没有最终输出时仍保留思考，并明确标记生成失败', async () => {
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(
    completion('', 'stop', '正在构思车架。')
  )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(await screen.findByText('正在构思车架。')).toBeVisible()
  expect(screen.getByRole('alert')).toHaveTextContent(
    'The model returned an empty result'
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    reasoningContent: '正在构思车架。',
    content: '',
    status: 'error',
  })
})

afterEach(async () => {
  vi.unstubAllGlobals()
  useAuthStore.getState().auth.reset()
  await i18next.changeLanguage('en')
})

it('选择生成模型后发送原始固定提示词，成功结果持久化并可显式加载隔离预览', async () => {
  const post = vi
    .spyOn(TestXMLHttpRequest, 'respond')
    .mockResolvedValue(completion(html))
  renderWorkspace()
  const submit = screen.getByRole('button', {
    name: 'Generate and save result',
  })
  await waitFor(() => expect(submit).toBeEnabled())
  expect(
    screen.getByRole('spinbutton', { name: 'Generation output limit (tokens)' })
  ).toHaveValue(16384)
  const generation = screen.getByRole('region', { name: 'Generation model' })
  await userEvent.click(within(generation).getAllByRole('combobox')[0])
  await userEvent.click(screen.getByRole('option', { name: 'model-b' }))
  await userEvent.click(submit)
  await screen.findByText('SVG detected')
  expect(post).toHaveBeenCalledWith(
    '/pg/chat/completions',
    {
      model: 'model-b',
      group: 'default',
      messages: [
        { role: 'system', content: expect.stringContaining('<!DOCTYPE html>') },
        { role: 'user', content: PELICAN_TEST_PROMPT },
      ],
      stream: true,
      max_tokens: 16384,
    },
    expect.any(Object)
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    model: 'model-b',
    status: 'success',
    content: html,
    maxTokens: 16384,
  })
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('tab', { name: 'Preview' }))
  await userEvent.click(screen.getByRole('button', { name: 'Load preview' }))
  const frame = screen.getByTitle('Pelican HTML preview')
  expect(frame).toHaveAttribute('sandbox', 'allow-scripts')
  expect(frame).toHaveAttribute('referrerpolicy', 'no-referrer')
  expect(frame.getAttribute('srcdoc')).toContain("connect-src 'none'")
  await userEvent.click(screen.getByRole('button', { name: 'Stop preview' }))
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
})

it('上游失败时保存失败历史及具体原因，并且不自动重试', async () => {
  const post = vi.spyOn(TestXMLHttpRequest, 'respond').mockRejectedValue({
    response: {
      data: { error: { message: 'unknown provider for model model-a' } },
    },
  })
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'unknown provider for model model-a'
  )
  expect(post).toHaveBeenCalledTimes(1)
  expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
    status: 'error',
    errorMessage: 'unknown provider for model model-a',
  })
  expect(
    screen.getByRole('button', { name: 'Assess current result' })
  ).toBeDisabled()
})

it('停止在途生成后保留取消历史，晚到的成功响应不会覆盖状态', async () => {
  let resolveResponse: (response: ReturnType<typeof completion>) => void = () =>
    undefined
  vi.spyOn(TestXMLHttpRequest, 'respond').mockImplementation(
    () =>
      new Promise((resolve) => {
        resolveResponse = resolve
      })
  )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(
    screen.getByRole('spinbutton', { name: 'Generation output limit (tokens)' })
  ).toBeDisabled()
  await userEvent.click(await screen.findByRole('button', { name: 'Stop' }))
  await act(async () => resolveResponse(completion(html)))
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0].status).toBe(
    'cancelled'
  )
  expect(screen.queryByText('SVG detected')).not.toBeInTheDocument()
})

it('选择评审模型后对当前结果生成评审，保存评审模型和历史并保留原始结果', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('old')])
  const post = vi
    .spyOn(TestXMLHttpRequest, 'respond')
    .mockResolvedValue(
      completion(
        '鹈鹕 8/10；代码包含循环 SVG 动画；实际流畅性未验证',
        'stop',
        '先检查 SVG 结构与动画属性。'
      )
    )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Assess current result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    within(screen.getByRole('region', { name: 'Assessment model' })).getByRole(
      'combobox'
    )
  )
  await userEvent.click(screen.getByRole('option', { name: 'model-b' }))
  await userEvent.click(
    screen.getByRole('button', { name: 'Assess current result' })
  )
  await screen.findByRole('region', { name: 'Assessment history' })
  expect(post).toHaveBeenCalledWith(
    '/pg/chat/completions',
    expect.objectContaining({
      model: 'model-b',
      stream: true,
      messages: [
        expect.objectContaining({ role: 'system' }),
        {
          role: 'user',
          content: JSON.stringify({
            task: PELICAN_TEST_PROMPT,
            generation_model: 'model-a',
            html,
          }),
        },
      ],
    }),
    expect.any(Object)
  )
  const saved = readPelicanTestHistory(historyStorageKey(17))[0]
  expect(saved.content).toBe(html)
  expect(saved.assessments[0]).toMatchObject({
    model: 'model-b',
    status: 'success',
    reasoningContent: '先检查 SVG 结构与动画属性。',
    content: '鹈鹕 8/10；代码包含循环 SVG 动画；实际流畅性未验证',
  })
  const history = screen.getByRole('region', { name: 'Assessment history' })
  await userEvent.click(within(history).getByText(/model-b/))
  expect(within(history).getByText('先检查 SVG 结构与动画属性。')).toBeVisible()
  expect(within(history).getByLabelText('Output content')).toHaveTextContent(
    '鹈鹕 8/10'
  )
})

it('勾选多条历史后只评审成功结果，并允许标记质量和对比选中结果', async () => {
  const failed = {
    ...savedRecord('failed'),
    status: 'error' as const,
    content: '',
  }
  writePelicanTestHistory(historyStorageKey(17), [
    savedRecord('a'),
    savedRecord('b', 'model-b'),
    failed,
  ])
  const post = vi
    .spyOn(TestXMLHttpRequest, 'respond')
    .mockResolvedValue(completion('代码评审结果'))
  renderWorkspace()
  await screen.findAllByRole('checkbox')
  for (const checkbox of screen.getAllByRole('checkbox')) {
    await userEvent.click(checkbox)
  }
  expect(
    await screen.findByRole('region', { name: 'Compare selected results' })
  ).toBeVisible()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Assess selected history (2)' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Assess selected history (2)' })
  )
  await waitFor(() =>
    expect(
      readPelicanTestHistory(historyStorageKey(17)).filter(
        (record) => record.assessments.length
      ).length
    ).toBe(2)
  )
  expect(post).toHaveBeenCalledTimes(2)
  await userEvent.click(
    screen.getAllByRole('button', { name: 'Good result' })[0]
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0].rating).toBe('good')
})

it('界面语言切换为简体与繁体中文时，日期显示和已保存历史保持可用', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('existing')])
  renderWorkspace()
  await screen.findByRole('region', { name: 'Result for model-a' })
  await act(async () => i18next.changeLanguage('zhCN'))
  expect(
    screen.getByRole('region', { name: 'Result for model-a' })
  ).toBeVisible()
  await act(async () => i18next.changeLanguage('zhTW'))
  expect(
    screen.getByRole('region', { name: 'Result for model-a' })
  ).toBeVisible()
})

it('存储写入失败时保留当前结果并提示未保存，而不是报告请求失败', async () => {
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(completion(html))
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
    throw new Error('storage quota')
  })
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  await screen.findByText('SVG detected')
  expect(screen.getByRole('alert')).toHaveTextContent(
    'History could not be saved'
  )
})

it('切换用户时不会显示或写入另一位用户的测试历史', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('alice')])
  writePelicanTestHistory(historyStorageKey(18), [
    savedRecord('bob', 'model-b'),
  ])
  renderWorkspace()
  await screen.findByRole('region', { name: 'Result for model-a' })
  act(() =>
    useAuthStore.getState().auth.setUser({ id: 18, username: 'bob', role: 1 })
  )
  await screen.findByRole('region', { name: 'Result for model-b' })
  expect(
    screen.queryByRole('region', { name: 'Result for model-a' })
  ).not.toBeInTheDocument()
  expect(readPelicanTestHistory(historyStorageKey(17))[0].id).toBe('alice')
})

it.each([
  ['HTML', '<html><svg>'],
  ['Canvas 链接', canvasResponse],
])(
  '模型返回截断 %s 时优先提示截断，保留内容且不提供预览或评审',
  async (_name, content) => {
    vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(
      completion(content, 'length')
    )
    renderWorkspace()
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Generate and save result' })
      ).toBeEnabled()
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Generate and save result' })
    )
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The model response was truncated'
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Increase the output limit and test again'
    )
    expect(screen.getByRole('alert')).not.toHaveTextContent(canvasGuidance)
    expect(readPelicanTestHistory(historyStorageKey(17))[0]).toMatchObject({
      status: 'error',
      content,
      finishReason: 'length',
    })
    expect(
      screen.queryByRole('button', { name: 'Load preview' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Assess current result' })
    ).toBeDisabled()
  }
)

it('模型列表获取失败时禁止提交，并允许重试获取列表', async () => {
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).endsWith('/models')) throw new Error('unavailable models')
    return {
      data: { success: true, data: { default: { desc: 'Default', ratio: 1 } } },
    }
  })
  const post = vi.spyOn(TestXMLHttpRequest, 'respond')
  renderWorkspace()
  await screen.findByText('Failed to load models')
  expect(
    screen.getByRole('button', { name: 'Generate and save result' })
  ).toBeDisabled()
  get.mockImplementation(async (url) => ({
    data: {
      success: true,
      data: String(url).endsWith('/models')
        ? ['model-a']
        : { default: { desc: 'Default', ratio: 1 } },
    },
  }))
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  expect(post).not.toHaveBeenCalled()
})

it('已有历史损坏时提示错误且禁止新任务覆盖原数据', async () => {
  localStorage.setItem(historyStorageKey(17), '{bad history')
  renderWorkspace()
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Test history could not be loaded'
  )
  expect(
    screen.getByRole('button', { name: 'Generate and save result' })
  ).toBeDisabled()
  expect(localStorage.getItem(historyStorageKey(17))).toBe('{bad history')
})

it('查看不同历史结果时不会沿用之前的预览执行状态', async () => {
  writePelicanTestHistory(historyStorageKey(17), [
    savedRecord('a'),
    savedRecord('b', 'model-b'),
  ])
  renderWorkspace()
  await userEvent.click(await screen.findByRole('tab', { name: 'Preview' }))
  await userEvent.click(
    await screen.findByRole('button', { name: 'Load preview' })
  )
  expect(screen.getByTitle('Pelican HTML preview')).toBeInTheDocument()
  await userEvent.click(
    screen.getByRole('button', { name: /model-b.*Completed/ })
  )
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
  expect(screen.getByRole('tab', { name: 'Output' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  await userEvent.click(screen.getByRole('tab', { name: 'Preview' }))
  expect(screen.getByRole('button', { name: 'Load preview' })).toBeEnabled()
})

it('旧历史没有思考字段时明确提示内容不可用，并保持原有输出可读', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('legacy')])
  renderWorkspace()
  expect(
    await screen.findByText('No reasoning content is available')
  ).toBeVisible()
  expect(screen.getByLabelText('Output content').textContent).toBe(html)
})

it('HTML 内部的 think 文本保留在输出里且默认按文本展示，不自动运行代码', async () => {
  const content =
    '<html><body><svg></svg><script>const value="<think>HTML 内的文本</think>";</script></body></html>'
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(
    completion(content, 'stop', '这是接口返回的思考。')
  )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Generate and save result' })
  )
  expect(await screen.findByText('这是接口返回的思考。')).toBeVisible()
  expect(screen.getByLabelText('Output content').textContent).toBe(content)
  expect(
    screen.getByLabelText('Output content').querySelector('script')
  ).toBeNull()
  expect(readPelicanTestHistory(historyStorageKey(17))[0].content).toBe(content)
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
})

it('配置输出上限后请求和历史保留该值，固定提示词不变', async () => {
  const post = vi
    .spyOn(TestXMLHttpRequest, 'respond')
    .mockResolvedValue(completion(html))
  renderWorkspace()
  const limit = await screen.findByRole('spinbutton', {
    name: 'Generation output limit (tokens)',
  })
  fireEvent.change(limit, { target: { value: '32768' } })
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeEnabled()
  )
  await userEvent.type(limit, '{enter}')
  await screen.findByText('SVG detected')
  expect(post).toHaveBeenCalledWith(
    '/pg/chat/completions',
    expect.objectContaining({
      max_tokens: 32768,
      messages: [
        { role: 'system', content: expect.stringContaining('<!DOCTYPE html>') },
        { role: 'user', content: PELICAN_TEST_PROMPT },
      ],
    }),
    expect.any(Object)
  )
  expect(readPelicanTestHistory(historyStorageKey(17))[0].maxTokens).toBe(32768)
  expect(screen.getByText('Output limit: 32,768 tokens')).toBeVisible()
})

it.each(['', '0', '-1', '1.5', '200001'])(
  '输出上限为 %s 时阻止生成，修正为有效整数后可提交',
  async (value) => {
    const post = vi
      .spyOn(TestXMLHttpRequest, 'respond')
      .mockResolvedValue(completion(html))
    renderWorkspace()
    const limit = await screen.findByRole('spinbutton', {
      name: 'Generation output limit (tokens)',
    })
    fireEvent.change(limit, { target: { value } })
    await waitFor(() => expect(limit).toHaveAttribute('aria-invalid', 'true'))
    expect(
      screen.getByRole('button', { name: 'Generate and save result' })
    ).toBeDisabled()
    expect(post).not.toHaveBeenCalled()
    fireEvent.change(limit, { target: { value: '2048' } })
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Generate and save result' })
      ).toBeEnabled()
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Generate and save result' })
    )
    await screen.findByText('SVG detected')
    expect(post).toHaveBeenCalledWith(
      '/pg/chat/completions',
      expect.objectContaining({ max_tokens: 2048 }),
      expect.any(Object)
    )
  }
)

it('绕过页面校验时也不向计费接口发送无效输出上限', async () => {
  const post = vi.spyOn(TestXMLHttpRequest, 'respond')
  for (const maxTokens of [Number.NaN, 1073741824]) {
    await expect(
      generatePelicanHtml(
        'model-a',
        'default',
        maxTokens,
        new AbortController().signal
      )
    ).rejects.toThrow('Invalid output token limit')
  }
  expect(post).not.toHaveBeenCalled()
})

it('评审被截断时保留思考与部分输出，并显示失败原因而不是当作完整评审', async () => {
  writePelicanTestHistory(historyStorageKey(17), [savedRecord('existing')])
  vi.spyOn(TestXMLHttpRequest, 'respond').mockResolvedValue(
    completion('部分评审结论', 'length', '检查动画循环。')
  )
  renderWorkspace()
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Assess current result' })
    ).toBeEnabled()
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Assess current result' })
  )
  const history = await screen.findByRole('region', {
    name: 'Assessment history',
  })
  await userEvent.click(within(history).getByText(/model-a/))
  expect(within(history).getByRole('alert')).toHaveTextContent(
    'The model response was truncated'
  )
  expect(within(history).getByText('检查动画循环。')).toBeVisible()
  expect(within(history).getByLabelText('Output content')).toHaveTextContent(
    '部分评审结论'
  )
  expect(
    readPelicanTestHistory(historyStorageKey(17))[0].assessments[0]
  ).toMatchObject({
    status: 'error',
    content: '部分评审结论',
    reasoningContent: '检查动画循环。',
    errorMessage: 'The model response was truncated',
  })
})
