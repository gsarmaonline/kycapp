import { describe, expect, it } from 'vitest'
import type { UsageCounter } from './api'
import {
  buildEntitlementCheckSeries,
  summarizeUsage,
  formatRelativeTime,
  actionLabel,
} from './observability'

function row(
  partial: Partial<UsageCounter> & Pick<UsageCounter, 'period_start' | 'dim1_value' | 'dim2_value' | 'count'>,
): UsageCounter {
  return {
    organisation_id: 'org',
    meter_key: 'entitlement.check',
    dim1_key: 'entitlement',
    dim2_key: 'result',
    updated_at: partial.period_start,
    ...partial,
  }
}

describe('buildEntitlementCheckSeries', () => {
  it('fills missing days and splits allow/deny', () => {
    const points = buildEntitlementCheckSeries(
      [
        row({
          period_start: '2026-07-20T00:00:00Z',
          dim1_value: 'api_access',
          dim2_value: 'allowed',
          count: 10,
        }),
        row({
          period_start: '2026-07-20T00:00:00Z',
          dim1_value: 'api_access',
          dim2_value: 'denied',
          count: 2,
        }),
        row({
          period_start: '2026-07-22T00:00:00Z',
          dim1_value: 'premium',
          dim2_value: 'denied',
          count: 5,
        }),
      ],
      { from: '2026-07-20T00:00:00Z', to: '2026-07-23T00:00:00Z' },
    )
    expect(points).toHaveLength(3)
    expect(points[0]).toEqual({ day: '2026-07-20', allowed: 10, denied: 2, total: 12 })
    expect(points[1]).toEqual({ day: '2026-07-21', allowed: 0, denied: 0, total: 0 })
    expect(points[2]).toEqual({ day: '2026-07-22', allowed: 0, denied: 5, total: 5 })
    expect(summarizeUsage(points)).toEqual({ allowed: 10, denied: 7, total: 17 })
  })

  it('filters by entitlement key', () => {
    const points = buildEntitlementCheckSeries(
      [
        row({
          period_start: '2026-07-20T00:00:00Z',
          dim1_value: 'api_access',
          dim2_value: 'allowed',
          count: 3,
        }),
        row({
          period_start: '2026-07-20T00:00:00Z',
          dim1_value: 'premium',
          dim2_value: 'denied',
          count: 9,
        }),
      ],
      {
        from: '2026-07-20T00:00:00Z',
        to: '2026-07-21T00:00:00Z',
        entitlementKey: 'premium',
      },
    )
    expect(points[0]).toEqual({ day: '2026-07-20', allowed: 0, denied: 9, total: 9 })
  })
})

describe('formatRelativeTime', () => {
  it('formats recent times', () => {
    const now = new Date('2026-07-29T12:00:00Z')
    expect(formatRelativeTime('2026-07-29T11:59:30Z', now)).toBe('just now')
    expect(formatRelativeTime('2026-07-29T11:30:00Z', now)).toBe('30m ago')
  })
})

describe('actionLabel', () => {
  it('maps known actions', () => {
    expect(actionLabel('api_key.created')).toBe('API key created')
    expect(actionLabel('custom.thing')).toBe('custom thing')
  })
})
