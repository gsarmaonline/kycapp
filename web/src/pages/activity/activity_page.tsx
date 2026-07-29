import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { listOrgActivity, type ActivityEvent } from '../../api'
import { ActivityFeed } from '../../components/ActivityFeed'
import { PageHeader } from '../../crud/ui'

export function ActivityPage() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<ActivityEvent[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      setError(null)
      setItems(null)
      try {
        const res = await listOrgActivity(orgId, 100)
        if (!cancelled) setItems(res.items)
      } catch (e) {
        if (!cancelled) {
          setItems([])
          setError(e instanceof Error ? e.message : 'Failed to load activity')
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [orgId])

  return (
    <section>
      <PageHeader title="Activity" />
      <p className="lede">
        Semantic changes for this organisation: plans, API keys, subscriptions, and related events.
      </p>
      {error ? <p className="error">{error}</p> : null}
      {items === null ? <p>Loading…</p> : <ActivityFeed items={items} />}
    </section>
  )
}
