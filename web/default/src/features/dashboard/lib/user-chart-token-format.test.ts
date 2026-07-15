import assert from 'node:assert/strict'
import { afterEach, describe, test } from 'node:test'

import { DEFAULT_SYSTEM_NAME, DEFAULT_LOGO } from '@/lib/constants'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { processChartData, processUserChartData } from './charts'
import type { QuotaDataItem } from '../types'

type TooltipFormatter = (datum: Record<string, unknown>) => string | number
type TooltipLineItem = { key: string; value: string | number }
type UserRankSpec = {
  title: { subtext: string }
  label: { formatMethod: (value: number) => string }
  tooltip: {
    mark: {
      content: Array<{ value: TooltipFormatter }>
    }
  }
}
type TokenLineSpec = {
  tooltip: {
    mark: {
      content: Array<{ value: TooltipFormatter }>
    }
    dimension: {
      updateContent: (array: TooltipLineItem[]) => TooltipLineItem[]
    }
  }
}
type TokenRankSpec = {
  tooltip: {
    mark: {
      content: Array<{ value: TooltipFormatter }>
    }
  }
}

const baseRows: QuotaDataItem[] = [
  {
    username: 'alice',
    model_name: 'gpt-4.1',
    quota: 413_000_000,
    token_used: 413_000_000,
    count: 1,
    created_at: 1782835200,
  },
  {
    username: 'bob',
    model_name: 'claude-sonnet-4',
    quota: 240_000_000,
    token_used: 240_000_000,
    count: 1,
    created_at: 1782921600,
  },
]

afterEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      systemName: DEFAULT_SYSTEM_NAME,
      logo: DEFAULT_LOGO,
      currency: { ...DEFAULT_CURRENCY_CONFIG },
    },
  }))
})

describe('dashboard token unit displays', () => {
  test('uses Chinese compact units for user token ranking values', () => {
    const result = processUserChartData(baseRows, 'day', (key) => key, 10, 'tokens')
    const rank = result.spec_user_rank as unknown as UserRankSpec

    assert.equal(rank.title.subtext, 'Total: 6.5亿')
    assert.equal(rank.label.formatMethod(413_000_000), '4.1亿')
    assert.equal(
      rank.tooltip.mark.content[0].value({ rawValue: 240_000_000 }),
      '2.4亿'
    )
  })

  test('uses Chinese compact units when quota display mode is tokens', () => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'TOKENS',
        },
      },
    }))

    const result = processUserChartData(baseRows, 'day', (key) => key, 10, 'amount')
    const rank = result.spec_user_rank as unknown as UserRankSpec

    assert.equal(rank.title.subtext, 'Total: 6.5亿')
    assert.equal(rank.label.formatMethod(413_000_000), '4.1亿')
    assert.equal(
      rank.tooltip.mark.content[0].value({ rawValue: 240_000_000 }),
      '2.4亿'
    )
  })

  test('uses Chinese compact units for model token chart tooltips', () => {
    const result = processChartData(baseRows, 'day', (key) => key)
    const line = result.spec_token_line as unknown as TokenLineSpec
    const rank = result.spec_token_rank_bar as unknown as TokenRankSpec
    const dimensionRows = [
      { key: 'alice', value: 413_000_000 },
      { key: 'bob', value: 240_000_000 },
    ]

    assert.equal(
      line.tooltip.mark.content[0].value({ Tokens: 413_000_000 }),
      '4.1亿'
    )
    assert.deepEqual(line.tooltip.dimension.updateContent(dimensionRows), [
      { key: 'Total:', value: '6.5亿' },
      { key: 'alice', value: '4.1亿' },
      { key: 'bob', value: '2.4亿' },
    ])
    assert.equal(
      rank.tooltip.mark.content[0].value({ Tokens: 240_000_000 }),
      '2.4亿'
    )
  })
})
