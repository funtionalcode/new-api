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
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { TestHistory } from '../components/test-history'
import {
  buildPreviewDocument,
  extractHtmlDocument,
  historyStorageKey,
  readPelicanTestHistory,
  writePelicanTestHistory,
  type PelicanTestRecord,
} from '../lib'

const record: PelicanTestRecord = {
  id: 'run',
  model: 'model-a',
  group: 'default',
  createdAt: 1720000000000,
  status: 'success',
  content: '<svg></svg>',
  assessments: [],
}

it('历史超过一页时可翻页查看，按模型筛选后回到第一页', async () => {
  const records = Array.from({ length: 11 }, (_, index) => ({
    ...record,
    id: `run-${index}`,
    model: index === 10 ? 'model-b' : 'model-a',
  }))
  render(
    <TestHistory
      records={records}
      selectedIds={[]}
      currentId={null}
      onView={vi.fn()}
      onSelect={vi.fn()}
    />
  )
  expect(screen.getAllByRole('checkbox')).toHaveLength(10)
  await userEvent.click(screen.getByRole('button', { name: 'Go to next page' }))
  expect(screen.getAllByRole('checkbox')).toHaveLength(1)
  expect(
    screen.getByRole('button', { name: /model-b.*Completed/ })
  ).toBeVisible()
  fireEvent.change(screen.getByRole('combobox', { name: 'Filter by model' }), {
    target: { value: 'model-a' },
  })
  expect(screen.getAllByRole('checkbox')).toHaveLength(10)
  expect(
    screen.getByRole('button', { name: 'Go to previous page' })
  ).toBeDisabled()
})

it('已选择四条历史时禁用其他选项并保留取消选中的能力', () => {
  const records = ['a', 'b', 'c', 'd', 'e'].map((id) => ({ ...record, id }))
  render(
    <TestHistory
      records={records}
      selectedIds={['a', 'b', 'c', 'd']}
      currentId={null}
      onView={vi.fn()}
      onSelect={vi.fn()}
    />
  )
  const options = screen.getAllByRole('checkbox')
  expect(options[4]).toHaveAttribute('aria-disabled', 'true')
  expect(options[0]).not.toHaveAttribute('aria-disabled', 'true')
  expect(options[0]).toBeChecked()
})

it('持久化保留所有历史和质量标记，不静默裁剪早期记录', () => {
  const records = Array.from({ length: 31 }, (_, index) => ({
    ...record,
    id: `run-${index}`,
    rating: 'good' as const,
  }))
  writePelicanTestHistory(historyStorageKey(1), records)
  expect(readPelicanTestHistory(historyStorageKey(1))).toEqual(records)
  expect(readPelicanTestHistory(historyStorageKey(2))).toEqual([])
})

it('损坏的历史被明确拒绝，原始存储不会被覆盖', () => {
  localStorage.setItem(historyStorageKey(1), '{invalid')
  expect(() => readPelicanTestHistory(historyStorageKey(1))).toThrow()
  expect(localStorage.getItem(historyStorageKey(1))).toBe('{invalid')
})

it('HTML 代码块优先于前面的说明文字，裸 SVG 也可预览', () => {
  expect(
    extractHtmlDocument(
      '说明\n```html\n<html><body>鹈鹕</body></html>\n```\n其他内容'
    )
  ).toBe('<html><body>鹈鹕</body></html>')
  expect(extractHtmlDocument('说明 <svg viewBox="0 0 20 20"></svg> 尾部')).toBe(
    '<svg viewBox="0 0 20 20"></svg>'
  )
})

it('预览策略放在所有生成内容之前，移除跳转和嵌套页面但保留动画脚本', () => {
  const source =
    '<html><head><meta http-equiv="refresh" content="0;url=https://example.com"><base href="https://example.com"><script>animate()</script></head><body><iframe src="/api/user/self"></iframe><svg></svg></body></html>'
  const document = new DOMParser().parseFromString(
    buildPreviewDocument(source),
    'text/html'
  )
  expect(document.head.firstElementChild?.getAttribute('http-equiv')).toBe(
    'Content-Security-Policy'
  )
  expect(document.head.firstElementChild?.getAttribute('content')).toContain(
    "default-src 'none'"
  )
  expect(document.head.firstElementChild?.getAttribute('content')).toContain(
    "form-action 'none'"
  )
  expect(document.querySelector('script')?.textContent).toBe('animate()')
  expect(
    document.querySelector('iframe, base, meta[http-equiv="refresh"]')
  ).toBeNull()
  expect(document.querySelector('svg')).not.toBeNull()
})
