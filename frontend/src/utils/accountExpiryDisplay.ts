import type { Account } from '@/types'

export type AccountExpirySource = 'account' | 'subscription'

export interface AccountExpiryDisplay {
  value: number | null
  source: AccountExpirySource | null
}

export const parseExpiryUnixSeconds = (value: unknown): number | null => {
  if (typeof value === 'number') {
    if (!Number.isFinite(value) || value <= 0) return null
    return Math.floor(value > 1_000_000_000_000 ? value / 1000 : value)
  }

  if (typeof value !== 'string') return null
  const trimmed = value.trim()
  if (!trimmed) return null

  if (/^\d+$/.test(trimmed)) {
    const numeric = Number(trimmed)
    if (!Number.isFinite(numeric) || numeric <= 0) return null
    return Math.floor(numeric > 1_000_000_000_000 ? numeric / 1000 : numeric)
  }

  const parsed = Date.parse(trimmed)
  if (!Number.isFinite(parsed)) return null
  return Math.floor(parsed / 1000)
}

export const resolveAccountExpiryDisplay = (
  account: Pick<Account, 'expires_at' | 'credentials'>
): AccountExpiryDisplay => {
  const accountExpiry = parseExpiryUnixSeconds(account.expires_at)
  if (accountExpiry) {
    return { value: accountExpiry, source: 'account' }
  }

  const subscriptionExpiry = parseExpiryUnixSeconds(account.credentials?.subscription_expires_at)
  if (subscriptionExpiry) {
    return { value: subscriptionExpiry, source: 'subscription' }
  }

  return { value: null, source: null }
}
