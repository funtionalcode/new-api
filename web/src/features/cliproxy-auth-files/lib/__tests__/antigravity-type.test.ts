import { describe, expect, test } from 'vitest'

import { toBindingFormData } from '../../api'
import {
  getCliproxyAuthFileEmail,
  getCliproxyAuthFileType,
  getCliproxyAuthFileTypeLabel,
} from '../auth-file-type'

describe('Antigravity 认证文件识别', () => {
  test('oauth 文件名识别为 Antigravity 并提取邮箱', () => {
    const file = {
      auth_name: 'antigravity-test@example.com.json',
      last_plan_type: 'oauth',
    }
    expect(getCliproxyAuthFileType(file)).toBe('antigravity')
    expect(getCliproxyAuthFileEmail(file)).toBe('test@example.com')
    expect(getCliproxyAuthFileTypeLabel(getCliproxyAuthFileType(file))).toBe(
      'Antigravity'
    )
  })

  test('绑定时保留明确的提供商，支持没有前缀的文件名', () => {
    const binding = toBindingFormData(
      {
        authIndex: 'ag',
        name: 'account.json',
        provider: 'antigravity',
        planType: 'oauth',
        enabled: true,
      },
      1
    )
    expect(getCliproxyAuthFileType(binding)).toBe('antigravity')
  })
})
