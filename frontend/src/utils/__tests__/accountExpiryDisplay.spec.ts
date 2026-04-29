import { describe, expect, it } from 'vitest'

import { parseExpiryUnixSeconds, resolveAccountExpiryDisplay } from '../accountExpiryDisplay'
import type { Account } from '@/types'

const makeAccount = (expiresAt: unknown, subscriptionExpiresAt?: unknown) => ({
  expires_at: expiresAt,
  credentials: subscriptionExpiresAt === undefined ? {} : {
    subscription_expires_at: subscriptionExpiresAt
  }
}) as Pick<Account, 'expires_at' | 'credentials'>

describe('accountExpiryDisplay', () => {
  it('prefers account expires_at over subscription expiry', () => {
    expect(resolveAccountExpiryDisplay(makeAccount(1_800_000_000, '2026-05-10T00:00:00Z'))).toEqual({
      value: 1_800_000_000,
      source: 'account'
    })
  })

  it('falls back to credentials.subscription_expires_at', () => {
    expect(resolveAccountExpiryDisplay(makeAccount(null, '2026-05-10T00:00:00Z'))).toEqual({
      value: Math.floor(Date.parse('2026-05-10T00:00:00Z') / 1000),
      source: 'subscription'
    })
  })

  it('returns no source when neither expiry exists', () => {
    expect(resolveAccountExpiryDisplay(makeAccount(null))).toEqual({
      value: null,
      source: null
    })
  })

  it('normalizes millisecond timestamps to seconds', () => {
    expect(parseExpiryUnixSeconds(1_800_000_000_000)).toBe(1_800_000_000)
    expect(parseExpiryUnixSeconds('1800000000000')).toBe(1_800_000_000)
  })
})
