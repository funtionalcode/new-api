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
import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import { getCliproxyAuthFileType } from '../auth-file-type'
import { getCliproxyPlanLabelConfig } from '../plan-label'

describe('Claude plan labels', () => {
  test('shows Claude Pro without Codex Plus or multiplier labels', () => {
    for (const value of ['claude_pro', 'plan_pro', 'plus']) {
      const config = getCliproxyPlanLabelConfig('claude', value)

      assert.equal(config?.label, 'Pro')
      assert.equal(config?.multiplier, undefined)
    }
  })

  test('distinguishes Max 5x and Max 20x rate-limit tiers', () => {
    const max5x = getCliproxyPlanLabelConfig('claude', 'default_claude_max_5x')
    const max20x = getCliproxyPlanLabelConfig('claude', 'claude_max_20x')

    assert.deepEqual(
      { label: max5x?.label, multiplier: max5x?.multiplier },
      { label: 'Max', multiplier: '5x' }
    )
    assert.deepEqual(
      { label: max20x?.label, multiplier: max20x?.multiplier },
      { label: 'Max', multiplier: '20x' }
    )
  })

  test('does not guess a Max multiplier for legacy values', () => {
    for (const value of ['claude_max', 'plan_max']) {
      const config = getCliproxyPlanLabelConfig('claude', value)

      assert.equal(config?.label, 'Max')
      assert.equal(config?.multiplier, undefined)
    }
  })

  test('keeps Codex plan labels unchanged', () => {
    assert.deepEqual(getCliproxyPlanLabelConfig('codex', 'plus')?.label, 'Plus')
    assert.deepEqual(
      {
        label: getCliproxyPlanLabelConfig('codex', 'prolite')?.label,
        multiplier: getCliproxyPlanLabelConfig('codex', 'prolite')?.multiplier,
      },
      { label: 'Pro', multiplier: '5x' }
    )
    assert.deepEqual(
      {
        label: getCliproxyPlanLabelConfig('codex', 'pro')?.label,
        multiplier: getCliproxyPlanLabelConfig('codex', 'pro')?.multiplier,
      },
      { label: 'Pro', multiplier: '20x' }
    )
  })

  test('recognizes canonical Claude plan values without a filename prefix', () => {
    for (const lastPlanType of [
      'claude_pro',
      'claude_max',
      'claude_max_5x',
      'claude_max_20x',
    ]) {
      assert.equal(
        getCliproxyAuthFileType({ last_plan_type: lastPlanType }),
        'claude'
      )
    }
  })

  test('keeps current xAI plan names distinct', () => {
    for (const plan of [
      'SuperGrok Lite',
      'SuperGrok',
      'SuperGrok Plus',
      'SuperGrok Heavy',
    ]) {
      assert.equal(getCliproxyPlanLabelConfig('xai', plan)?.label, plan)
    }
  })
})
