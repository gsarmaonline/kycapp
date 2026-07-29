import { useEffect, useMemo, useState } from 'react'
import { listOrgUsage, type UsageCounter } from '../api'
import {
  buildEntitlementCheckSeries,
  daysAgoUTC,
  summarizeUsage,
  tomorrowUTC,
} from '../observability'
import { StackedBarChart } from './StackedBarChart'

export function EntitlementUsageChart({
  orgId,
  entitlementKey,
  days = 14,
  title = 'Entitlement checks',
}: {
  orgId: string
  entitlementKey?: string
  days?: number
  title?: string
}) {
  const [rows, setRows] = useState<UsageCounter[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const from = useMemo(() => daysAgoUTC(days - 1), [days])
  const to = useMemo(() => tomorrowUTC(), [])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      setError(null)
      try {
        const res = await listOrgUsage(orgId, { from, to })
        if (!cancelled) setRows(res.items)
      } catch (e) {
        if (!cancelled) {
          setRows([])
          setError(e instanceof Error ? e.message : 'Failed to load usage')
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [orgId, from, to])

  const points = useMemo(
    () =>
      buildEntitlementCheckSeries(rows ?? [], {
        from,
        to,
        entitlementKey,
      }),
    [rows, from, to, entitlementKey],
  )
  const summary = summarizeUsage(points)

  return (
    <section className="obs-card">
      <header className="obs-card-header">
        <h3 className="obs-card-title">{title}</h3>
        <p className="obs-card-sub">Last {days} days (UTC)</p>
      </header>
      {error ? <p className="error">{error}</p> : null}
      {rows === null ? (
        <p className="lede">Loading…</p>
      ) : (
        <>
          <div className="obs-summary">
            <div>
              <strong>{summary.total}</strong>
              <span>checks</span>
            </div>
            <div>
              <strong>{summary.allowed}</strong>
              <span>allowed</span>
            </div>
            <div>
              <strong>{summary.denied}</strong>
              <span>denied</span>
            </div>
          </div>
          <StackedBarChart points={points} />
        </>
      )}
    </section>
  )
}
