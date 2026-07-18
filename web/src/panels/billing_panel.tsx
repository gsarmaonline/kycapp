import { useEffect, useState } from 'react'
import {
  createBillingCheckout,
  createBillingPortal,
  getOrgEntitlements,
  getSubscription,
  listPlanPrices,
  listPlans,
  type Plan,
  type PlanPrice,
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
  const [pricesByPlan, setPricesByPlan] = useState<Record<string, PlanPrice[]>>({})
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [effective, setEffective] = useState<string[]>([])
  const [busy, setBusy] = useState(false)

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
      const priceEntries = await Promise.all(
        p.items.map(async (plan) => {
          try {
            const res = await listPlanPrices(plan.id)
            return [plan.id, res.items] as const
          } catch {
            return [plan.id, [] as PlanPrice[]] as const
          }
        }),
      )
      setPricesByPlan(Object.fromEntries(priceEntries))
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Billing load failed')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  const currentPlan = plans.find((p) => p.id === subscription?.plan_id)
  const sellable = plans.filter((plan) => {
    const prices = pricesByPlan[plan.id] ?? []
    return plan.status === 'active' && prices.some((pr) => pr.status === 'active')
  })

  async function onCheckout(planId: string) {
    setBusy(true)
    onError(null)
    try {
      const { url } = await createBillingCheckout(orgId, { plan_id: planId, interval: 'month' })
      window.location.href = url
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Checkout failed')
      setBusy(false)
    }
  }

  async function onPortal() {
    setBusy(true)
    onError(null)
    try {
      const { url } = await createBillingPortal(orgId)
      window.location.href = url
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Billing portal failed')
      setBusy(false)
    }
  }

  return (
    <section className="billing">
      <p className="status">
        Subscription: {subscription ? `${subscription.status}` : 'none'}
        {currentPlan ? ` · ${currentPlan.name} (${currentPlan.key})` : ''}
        {subscription?.current_period_end
          ? ` · period ends ${new Date(subscription.current_period_end).toLocaleDateString()}`
          : ''}
      </p>
      <p>Effective entitlements: {effective.length ? effective.join(', ') : 'none'}</p>

      <div className="row" style={{ gap: '0.75rem', flexWrap: 'wrap', marginTop: '1rem' }}>
        {sellable.map((plan) => {
          const isCurrent = plan.id === subscription?.plan_id
          return (
            <button
              key={plan.id}
              type="button"
              disabled={busy || isCurrent}
              onClick={() => void onCheckout(plan.id)}
            >
              {isCurrent ? `Current: ${plan.name}` : `Upgrade to ${plan.name}`}
            </button>
          )
        })}
        <button type="button" disabled={busy} onClick={() => void onPortal()}>
          Manage billing
        </button>
      </div>
      <p className="lede" style={{ marginTop: '1rem' }}>
        Card details are collected on Stripe Checkout / Customer Portal. KYC only starts those
        sessions and updates your plan from webhooks.
      </p>
    </section>
  )
}
