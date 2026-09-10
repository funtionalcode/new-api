import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { expect, test } from 'vitest'

import { AntigravityUsageCell } from '../antigravity-usage-cell'

test.each(['zhCN', 'zhTW'])(
  '使用项目语言码 %s 时可正常展示 Antigravity 额度',
  async (language) => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: language,
      fallbackLng: false,
      resources: {
        [language]: {
          translation: { 'Used {{percent}}%': '已用 {{percent}}%' },
        },
      },
      interpolation: { escapeValue: false },
    })
    render(
      <I18nextProvider i18n={i18n}>
        <AntigravityUsageCell
          binding={{
            last_error: '',
            last_antigravity_quota:
              '[{"bucket_id":"gemini-5h","remaining_fraction":0.9917125,"reset_at":0}]',
          }}
        />
      </I18nextProvider>
    )

    expect(screen.getByText('0.83%')).toBeInTheDocument()
    expect(
      Number(screen.getByRole('progressbar').getAttribute('aria-valuenow'))
    ).toBeCloseTo(0.82875)
  }
)
