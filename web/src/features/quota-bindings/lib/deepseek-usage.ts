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
import { formatTokenDetails, formatTokens } from '@/lib/format'

type JsonObject = Record<string, unknown>

type DeepSeekMoneyUsageInput = {
  normalWallets?: string
  bonusWallets?: string
  monthlyCosts?: string
  todayCosts?: string
  totalCosts?: string
  monthlyUsedTokens?: number
  requestCount?: number
}

export type DeepSeekMoneyUsage = {
  currency: string
  remainingAmount: number
  monthlyCostAmount: number
  todayCostAmount: number
  totalCostAmount: number
  monthlyUsedTokens: number
  requestCount: number
  totalAmount: number
  remainingPercent: number
  remainingLabel: string
  monthlyCostLabel: string
  todayCostLabel: string
  totalCostLabel: string
  monthlyTokenLabel: string
  monthlyTokenDetail: string
}

function parseJsonList(value: string | undefined): JsonObject[] {
  if (!value?.trim()) return []
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed)
      ? parsed.filter((item) => item && typeof item === 'object')
      : []
  } catch {
    return []
  }
}

function numericAmount(value: unknown): number {
  const amount = Number(value || 0)
  return Number.isFinite(amount) ? amount : 0
}

function firstCurrency(items: JsonObject[]): string {
  const item = items.find((entry) => typeof entry.currency === 'string')
  return typeof item?.currency === 'string' && item.currency.trim()
    ? item.currency.trim()
    : ''
}

function sumField(items: JsonObject[], field: string): number {
  return items.reduce((sum, item) => sum + numericAmount(item[field]), 0)
}

function formatAmount(value: number): string {
  return value.toLocaleString(undefined, {
    maximumFractionDigits: 4,
  })
}

function formatCurrencyAmount(currency: string, value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '-'
  return `${currency || '-'} ${formatAmount(value)}`
}

export function buildDeepSeekMoneyUsage(
  input: DeepSeekMoneyUsageInput
): DeepSeekMoneyUsage {
  const wallets = [
    ...parseJsonList(input.normalWallets),
    ...parseJsonList(input.bonusWallets),
  ]
  const monthlyCosts = parseJsonList(input.monthlyCosts)
  const todayCosts = parseJsonList(input.todayCosts)
  const totalCosts = parseJsonList(input.totalCosts)
  const currency =
    firstCurrency(wallets) ||
    firstCurrency(totalCosts) ||
    firstCurrency(monthlyCosts) ||
    firstCurrency(todayCosts)
  const remainingAmount = sumField(wallets, 'balance')
  const monthlyCostAmount = sumField(monthlyCosts, 'amount')
  const todayCostAmount = sumField(todayCosts, 'amount')
  const totalCostAmount = sumField(totalCosts, 'amount')
  const monthlyUsedTokens = Math.max(0, Number(input.monthlyUsedTokens || 0))
  const requestCount = Math.max(0, Number(input.requestCount || 0))
  const totalAmount =
    remainingAmount +
    (totalCostAmount > 0 ? totalCostAmount : monthlyCostAmount)
  const remainingPercent =
    totalAmount > 0
      ? Math.min(
          100,
          Math.max(0, Math.round((remainingAmount / totalAmount) * 100))
        )
      : 0

  return {
    currency,
    remainingAmount,
    monthlyCostAmount,
    todayCostAmount,
    totalCostAmount,
    monthlyUsedTokens,
    requestCount,
    totalAmount,
    remainingPercent,
    remainingLabel: formatCurrencyAmount(currency, remainingAmount),
    monthlyCostLabel: formatCurrencyAmount(currency, monthlyCostAmount),
    todayCostLabel: formatCurrencyAmount(currency, todayCostAmount),
    totalCostLabel: formatCurrencyAmount(currency, totalCostAmount),
    monthlyTokenLabel:
      monthlyUsedTokens > 0 ? formatTokens(monthlyUsedTokens) : '-',
    monthlyTokenDetail:
      monthlyUsedTokens > 0 ? formatTokenDetails(monthlyUsedTokens) : '-',
  }
}
