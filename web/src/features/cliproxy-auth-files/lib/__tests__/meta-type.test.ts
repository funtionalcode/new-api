import { describe, expect, test } from 'vitest'

import { toBindingFormData } from '../../api'
import {
  getCliproxyAuthFileEmail,
  getCliproxyAuthFileType,
  getCliproxyAuthFileTypeLabel,
} from '../auth-file-type'
import { getCliproxyPlanLabelConfig } from '../plan-label'

describe('Meta 认证文件识别', () => {
  test('识别当前提供商并保留绑定信息', () => {
    const binding = toBindingFormData(
      {
        authIndex: 'meta-index',
        name: 'account.json',
        provider: 'meta',
        enabled: true,
      },
      1
    )
    expect(getCliproxyAuthFileType(binding)).toBe('meta')
    expect(getCliproxyAuthFileTypeLabel(getCliproxyAuthFileType(binding))).toBe(
      'Muse (Meta)'
    )
  })

  test('支持文件名前缀，但明确提供商优先', () => {
    const source = { auth_name: 'meta-user@example.com.json' }
    expect(getCliproxyAuthFileType(source)).toBe('meta')
    expect(getCliproxyAuthFileEmail(source)).toBe('user@example.com')
    expect(getCliproxyAuthFileType({ ...source, provider: 'codex' })).toBe(
      'codex'
    )
  })

  test('不把提供商名当套餐，也不套用 Codex 倍率', () => {
    expect(getCliproxyPlanLabelConfig('meta', 'meta')).toBeNull()
    expect(getCliproxyPlanLabelConfig('meta', 'oauth')).toBeNull()
    expect(
      getCliproxyPlanLabelConfig('meta', 'Pro')?.multiplier
    ).toBeUndefined()
  })
})
