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
import { expect, test } from 'vitest'

import {
  getPlaygroundGenerationMode,
  getPlaygroundImageSize,
} from '../playground-task-models'

test.each(['qwen-image-2.1', 'Qwen/Qwen-Image-2.1'])(
  'routes %s to image generation with the validated local size',
  (model) => {
    expect(getPlaygroundGenerationMode('chat', model)).toBe('image')
    expect(getPlaygroundImageSize(model)).toBe('512x512')
  }
)

test('preserves explicit modes and unrelated model sizes', () => {
  expect(getPlaygroundGenerationMode('video', 'qwen-image-2.1')).toBe('video')
  expect(getPlaygroundGenerationMode('chat', 'qwen3.8-27b')).toBe('chat')
  expect(getPlaygroundGenerationMode('chat', 'qwen-image-2.10')).toBe('chat')
  expect(getPlaygroundImageSize('grok-imagine-image')).toBe('1024x1024')
  expect(getPlaygroundImageSize('qwen-image-2.10')).toBe('1024x1024')
})
