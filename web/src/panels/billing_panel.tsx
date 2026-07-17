import { useEffect, useState } from 'react'
import {
  getOrgEntitlements,
  getSubscription,
  listEntitlementsCatalog,
  listPlans,
  type Plan,
  type Subscription,
} from '../api'

export function BillingPanel({
  orgId,
  onError,
}: {
  orgId: string
  onError: (msg: string | null) => void
}) {
  const [plans, setPlans] = useState<Plan[]>([])
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [effective, setEffective] = useState<string[]>([])

  async function refresh() {
    onError(null)
    try {
      const [p, e] = await Promise.all([listPlans(), getOrgEntitlements(orgId)])
      setPlans(p.items)
      setEffective(e.entitlements)
      try {
        const sub = await getSubscription(orgId)
        setSubscription(sub)
      } catch {
        setSubscription(null)
      }
      await listEntitlementsCatalog()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Billing load failed')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  const currentPlan = plans.find((p) => p.id === subscription?.plan_id)

  return (
    <section className="billing">
      <p className="status">
        Subscription: {subscription ? `${subscription.status}` : 'none'}
        {currentPlan ? ` · ${currentPlan.name} (${currentPlan.key})` : ''}
      </p>
      <p>Effective entitlements: {effective.length ? effective.join(', ') : 'none'}</p>
      <p className="lede">Plan changes are managed by platform admins until self-serve billing ships.</p>
    </section>
  )
}
