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
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { ResultCard } from '../components/result-card'

const record = {
  id: 'preview',
  model: 'model-a',
  group: 'default',
  createdAt: 1720000000000,
  status: 'success' as const,
  content: '<html><body><svg viewBox="0 0 960 540"></svg></body></html>',
  assessments: [],
}
const requestDescriptor = Object.getOwnPropertyDescriptor(
  HTMLElement.prototype,
  'requestFullscreen'
)
const exitDescriptor = Object.getOwnPropertyDescriptor(
  document,
  'exitFullscreen'
)
const fullscreenDescriptor = Object.getOwnPropertyDescriptor(
  document,
  'fullscreenElement'
)
let fullscreenElement: Element | null = null
function setBrowserFullscreen(element: Element | null) {
  fullscreenElement = element
  document.dispatchEvent(new Event('fullscreenchange'))
}
const requestFullscreen = vi.fn<() => Promise<void>>()
const exitFullscreen = vi.fn(() => {
  setBrowserFullscreen(null)
  return Promise.resolve()
})

beforeEach(() => {
  fullscreenElement = null
  requestFullscreen.mockImplementation(function (this: HTMLElement) {
    setBrowserFullscreen(this)
    return Promise.resolve()
  })
  Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', {
    value: requestFullscreen,
    configurable: true,
  })
  Object.defineProperty(document, 'exitFullscreen', {
    value: exitFullscreen,
    configurable: true,
  })
  Object.defineProperty(document, 'fullscreenElement', {
    get: () => fullscreenElement,
    configurable: true,
  })
})
afterEach(() => {
  if (requestDescriptor) {
    Object.defineProperty(
      HTMLElement.prototype,
      'requestFullscreen',
      requestDescriptor
    )
  } else {
    Reflect.deleteProperty(HTMLElement.prototype, 'requestFullscreen')
  }
  if (exitDescriptor) {
    Object.defineProperty(document, 'exitFullscreen', exitDescriptor)
  } else {
    Reflect.deleteProperty(document, 'exitFullscreen')
  }
  if (fullscreenDescriptor) {
    Object.defineProperty(document, 'fullscreenElement', fullscreenDescriptor)
  } else {
    Reflect.deleteProperty(document, 'fullscreenElement')
  }
})

async function loadPreview() {
  render(<ResultCard record={record} onRatingChange={vi.fn()} />)
  await userEvent.click(screen.getByRole('tab', { name: 'Preview' }))
  await userEvent.click(screen.getByRole('button', { name: 'Load preview' }))
  return screen.getByTitle('Pelican HTML preview')
}

it('加载后动画视口由 CSS 固定为 1280×720，外层滚动查看而不缩小 iframe', async () => {
  const frame = await loadPreview()
  const preview = screen.getByRole('region', { name: 'Pelican HTML preview' })
  expect(preview).toHaveClass('h-[780px]')
  expect(frame).toHaveClass('h-[720px]', 'w-[1280px]', 'max-w-none')
  expect(screen.getByRole('region', { name: 'Preview' })).toHaveClass(
    'overflow-auto'
  )
  expect(frame).not.toHaveClass('h-80')
  expect(frame).toHaveAttribute('sandbox', 'allow-scripts')
  expect(frame.getAttribute('srcdoc')).toContain("connect-src 'none'")
})

it('键盘进入和退出原生全屏时保留同一个 iframe，不重载正在播放的动画', async () => {
  const user = userEvent.setup()
  const frame = await loadPreview()
  screen.getByRole('button', { name: 'Fullscreen preview' }).focus()
  await user.keyboard('{Enter}')
  expect(requestFullscreen).toHaveBeenCalledTimes(1)
  const exit = await screen.findByRole('button', { name: 'Exit fullscreen' })
  expect(screen.getByTitle('Pelican HTML preview')).toBe(frame)
  expect(frame).toHaveClass('h-[720px]', 'w-[1280px]')
  await user.click(exit)
  expect(exitFullscreen).toHaveBeenCalledTimes(1)
  expect(
    await screen.findByRole('button', { name: 'Fullscreen preview' })
  ).toBeEnabled()
  expect(screen.getByTitle('Pelican HTML preview')).toBe(frame)
})

it('浏览器通过 Escape 退出全屏时入口状态同步恢复，停止预览会卸载动画', async () => {
  const frame = await loadPreview()
  await userEvent.click(
    screen.getByRole('button', { name: 'Fullscreen preview' })
  )
  await screen.findByRole('button', { name: 'Exit fullscreen' })
  act(() => {
    fullscreenElement = null
    document.dispatchEvent(new Event('fullscreenchange'))
  })
  expect(
    screen.getByRole('button', { name: 'Fullscreen preview' })
  ).toBeEnabled()
  expect(screen.getByTitle('Pelican HTML preview')).toBe(frame)
  await userEvent.click(screen.getByRole('button', { name: 'Stop preview' }))
  expect(screen.queryByTitle('Pelican HTML preview')).not.toBeInTheDocument()
})

it.each(['unsupported', 'rejected'])(
  '全屏接口 %s 时降级为屏幕大小的弹窗，保持隔离且只运行一个 iframe',
  async (mode) => {
    if (mode === 'unsupported') {
      Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', {
        value: undefined,
        configurable: true,
      })
    } else {
      requestFullscreen.mockRejectedValue(new Error('Fullscreen unavailable'))
    }
    await loadPreview()
    await userEvent.click(
      screen.getByRole('button', { name: 'Fullscreen preview' })
    )
    const dialog = await screen.findByRole('dialog', {
      name: 'Pelican HTML preview',
    })
    const frame = within(dialog).getByTitle('Pelican HTML preview')
    expect(screen.getAllByTitle('Pelican HTML preview')).toHaveLength(1)
    expect(frame).toHaveAttribute('sandbox', 'allow-scripts')
    expect(frame).toHaveAttribute('referrerpolicy', 'no-referrer')
    expect(frame).toHaveClass('h-[720px]', 'w-[1280px]')
    expect(frame.getAttribute('srcdoc')).toContain("connect-src 'none'")
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Exit fullscreen' })
    )
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(screen.getAllByTitle('Pelican HTML preview')).toHaveLength(1)
    expect(
      screen.getByRole('button', { name: 'Fullscreen preview' })
    ).toHaveFocus()
  }
)

it.each(['native', 'dialog'])(
  '在 %s 全屏预览中点击停止会退出全屏并卸载动画',
  async (mode) => {
    if (mode === 'dialog') {
      Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', {
        value: undefined,
        configurable: true,
      })
    }
    await loadPreview()
    await userEvent.click(
      screen.getByRole('button', { name: 'Fullscreen preview' })
    )
    const scope =
      mode === 'dialog'
        ? await screen.findByRole('dialog')
        : screen.getByRole('region', { name: 'Pelican HTML preview' })
    await userEvent.click(
      within(scope).getByRole('button', { name: 'Stop preview' })
    )
    await waitFor(() =>
      expect(
        screen.queryByTitle('Pelican HTML preview')
      ).not.toBeInTheDocument()
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(fullscreenElement).toBeNull()
  }
)
