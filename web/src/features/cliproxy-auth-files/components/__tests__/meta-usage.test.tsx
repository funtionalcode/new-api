import { act, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { expect, test } from 'vitest'

import { MetaUsageCell } from '../meta-usage-cell'

test.each([
  ['zhCN', '15.13'],
  ['zhTW', '15.13'],
  ['en', '15.13'],
  ['fr', '15,13'],
  ['ru', '15,13'],
  ['ja', '15.13'],
  ['vi', '15,13'],
  [
    'invalid_locale',
    new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(
      15.125
    ),
  ],
])(
  'Meta 支持动态窗口、精确零值和项目语言码 %s',
  async (language, formattedPercent) => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: language,
      fallbackLng: false,
      resources: { [language]: { translation: {} } },
      interpolation: { escapeValue: false },
    })
    render(
      <I18nextProvider i18n={i18n}>
        <MetaUsageCell
          binding={{
            last_error: '',
            last_meta_quota: JSON.stringify({
              window: { used_percent: 0, duration_minutes: 120 },
              weekly: { used_percent: 15.125, reset_at: 1793012400 },
            }),
          }}
        />
      </I18nextProvider>
    )
    expect(screen.getByText('120-Minute Window')).toBeInTheDocument()
    expect(screen.getByText('0%')).toBeInTheDocument()
    expect(screen.getByText(`${formattedPercent}%`)).toBeInTheDocument()
    expect(
      screen
        .getAllByRole('progressbar')
        .map((bar) => bar.getAttribute('aria-valuenow'))
    ).toEqual(['0', '15.125'])
    expect(screen.queryByText('5-Hour Window')).not.toBeInTheDocument()
  }
)

test('Meta 额度格式随界面语言切换更新', async () => {
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: false,
    resources: { en: { translation: {} }, fr: { translation: {} } },
    interpolation: { escapeValue: false },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <MetaUsageCell
        binding={{
          last_error: '',
          last_meta_quota: '{"window":{"used_percent":15.125},"weekly":{}}',
        }}
      />
    </I18nextProvider>
  )
  expect(screen.getByText('15.13%')).toBeInTheDocument()
  await act(async () => {
    await i18n.changeLanguage('fr')
  })
  expect(screen.getByText('15,13%')).toBeInTheDocument()
  expect(screen.queryByText('15.13%')).not.toBeInTheDocument()
})

test('缺失额度保持未知，不显示虚构的进度', () => {
  render(
    <MetaUsageCell
      binding={{ last_error: '', last_meta_quota: '{"window":{},"weekly":{}}' }}
    />
  )
  expect(screen.getAllByText('Unknown')).toHaveLength(2)
  expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
})

test('刷新失败只显示错误，不显示旧额度', () => {
  render(
    <MetaUsageCell
      binding={{
        last_error:
          'Meta authorization expired or was denied. Sign in to Meta again.',
        last_meta_quota: '{"window":{"used_percent":20},"weekly":{}}',
      }}
    />
  )
  expect(screen.getByRole('alert')).toHaveTextContent('Sign in to Meta again.')
  expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
})
