import type { UsageCounter } from './api'

export type DailyUsagePoint = {
  /** UTC day YYYY-MM-DD */
  day: string
  allowed: number
  denied: number
  total: number
}

export const METER_ENTITLEMENT_CHECK = 'entitlement.check'

/** UTC midnight ISO for N days ago (inclusive start of that day). */
export function daysAgoUTC(days: number, now = new Date()): string {
  const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
  d.setUTCDate(d.getUTCDate() - days)
  return d.toISOString()
}

/** Exclusive end: tomorrow UTC midnight. */
export function tomorrowUTC(now = new Date()): string {
  const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
  d.setUTCDate(d.getUTCDate() + 1)
  return d.toISOString()
}

function dayKey(iso: string): string {
  return iso.slice(0, 10)
}

function enumerateDays(fromISO: string, toExclusiveISO: string): string[] {
  const out: string[] = []
  const cur = new Date(fromISO)
  const end = new Date(toExclusiveISO)
  cur.setUTCHours(0, 0, 0, 0)
  end.setUTCHours(0, 0, 0, 0)
  while (cur < end) {
    out.push(cur.toISOString().slice(0, 10))
    cur.setUTCDate(cur.getUTCDate() + 1)
  }
  return out
}

/**
 * Build a continuous daily series of entitlement checks (allow vs deny).
 * Optionally filter to a single entitlement key (dim1_value).
 */
export function buildEntitlementCheckSeries(
  rows: UsageCounter[],
  opts: { from: string; to: string; entitlementKey?: string },
): DailyUsagePoint[] {
  const days = enumerateDays(opts.from, opts.to)
  const byDay = new Map<string, { allowed: number; denied: number }>()
  for (const day of days) {
    byDay.set(day, { allowed: 0, denied: 0 })
  }

  for (const row of rows) {
    if (row.meter_key !== METER_ENTITLEMENT_CHECK) continue
    if (opts.entitlementKey && row.dim1_value !== opts.entitlementKey) continue
    const day = dayKey(row.period_start)
    const bucket = byDay.get(day)
    if (!bucket) continue
    if (row.dim2_value === 'denied') {
      bucket.denied += row.count
    } else {
      bucket.allowed += row.count
    }
  }

  return days.map((day) => {
    const b = byDay.get(day) ?? { allowed: 0, denied: 0 }
    return {
      day,
      allowed: b.allowed,
      denied: b.denied,
      total: b.allowed + b.denied,
    }
  })
}

export function summarizeUsage(points: DailyUsagePoint[]) {
  let allowed = 0
  let denied = 0
  for (const p of points) {
    allowed += p.allowed
    denied += p.denied
  }
  return { allowed, denied, total: allowed + denied }
}

export function formatRelativeTime(iso: string, now = new Date()): string {
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return iso
  const diffSec = Math.round((now.getTime() - t) / 1000)
  if (diffSec < 60) return 'just now'
  const mins = Math.round(diffSec / 60)
  if (mins < 60) return `${mins}m ago`
  const hours = Math.round(mins / 60)
  if (hours < 48) return `${hours}h ago`
  const days = Math.round(hours / 24)
  if (days < 14) return `${days}d ago`
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function actionLabel(action: string): string {
  const labels: Record<string, string> = {
    'org.created': 'Organisation created',
    'api_key.created': 'API key created',
    'api_key.revoked': 'API key revoked',
    'subscription.created': 'Subscription created',
    'subscription.updated': 'Subscription updated',
    'subscription.status_changed': 'Subscription status changed',
  }
  return labels[action] ?? action.replaceAll('.', ' ')
}
