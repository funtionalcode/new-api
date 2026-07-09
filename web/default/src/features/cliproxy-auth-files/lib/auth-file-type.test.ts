import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getCliproxyAuthFileEmail,
  getCliproxyAuthFileType,
} from './auth-file-type'

describe('cliproxy auth file type', () => {
  test('detects Claude bindings from auth names and plan types', () => {
    assert.equal(
      getCliproxyAuthFileType({
        auth_name: 'claude-gooddgege@gmail.com.json',
        auth_file: '',
        last_plan_type: '',
      }),
      'claude'
    )
    assert.equal(
      getCliproxyAuthFileType({
        auth_name: 'account.json',
        auth_file: '',
        last_plan_type: 'plan_max',
      }),
      'claude'
    )
  })

  test('defaults non-Claude bindings to Codex', () => {
    assert.equal(
      getCliproxyAuthFileType({
        auth_name: 'codex-hermensdriggars@gmail.com-pro.json',
        auth_file: '',
        last_plan_type: 'pro',
      }),
      'codex'
    )
  })

  test('detects Codex remote files from auth names', () => {
    assert.equal(
      getCliproxyAuthFileType({
        auth_name: 'codex-yuyangsga@gmail.com-pro.json',
        auth_file: '',
        last_plan_type: '',
      }),
      'codex'
    )
  })

  test('detects xAI remote files from auth names', () => {
    assert.equal(
      getCliproxyAuthFileType({
        auth_name: 'xai-gooddgege@gmail.com.json',
        auth_file: '',
        last_plan_type: '',
      }),
      'xai'
    )
  })

  test('extracts email from auth file names', () => {
    assert.equal(
      getCliproxyAuthFileEmail({
        auth_name: 'codex-hermensdriggars@gmail.com-pro.json',
        auth_file: '',
        last_plan_type: '',
      }),
      'hermensdriggars@gmail.com'
    )
    assert.equal(
      getCliproxyAuthFileEmail({
        auth_name: 'claude-gooddgege@gmail.com.json',
        auth_file: '',
        last_plan_type: '',
      }),
      'gooddgege@gmail.com'
    )
    assert.equal(
      getCliproxyAuthFileEmail({
        auth_name: 'xai-gooddgege@gmail.com.json',
        auth_file: '',
        last_plan_type: '',
      }),
      'gooddgege@gmail.com'
    )
  })
})
