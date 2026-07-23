import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  getActiveProductPlan,
  getProductPlan,
  setActiveProductPlan,
  type ProductPlan,
} from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'

export function ProductPlansShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<ProductPlan | null>(null)
  const [isActive, setIsActive] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function refresh() {
    const plan = await getProductPlan(id)
    setItem(plan)
    try {
      const active = await getActiveProductPlan(orgId)
      setIsActive(active.id === plan.id)
    } catch {
      setIsActive(false)
    }
  }

  useEffect(() => {
    void refresh().catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
  }, [id, orgId])

  async function onActivate() {
    setBusy(true)
    setError(null)
    try {
      await setActiveProductPlan(orgId, id)
      setIsActive(true)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Activate failed')
    } finally {
      setBusy(false)
    }
  }

  if (error && !item) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title={item.name} />
      {error && <p className="error">{error}</p>}
      <p className="row" style={{ gap: '0.75rem', flexWrap: 'wrap' }}>
        <Link className="button ghost" to={resourcePath(orgId, 'product-plans', item.id, 'edit')}>
          Edit
        </Link>
        <button type="button" disabled={busy || isActive || item.status !== 'active'} onClick={() => void onActivate()}>
          {isActive ? 'Active for product users' : 'Activate for product users'}
        </button>
        <Link className="button ghost" to={orgPath(orgId, 'billing')}>
          Billing
        </Link>
        <Link className="button ghost" to={resourcePath(orgId, 'product-plans')}>
          Back
        </Link>
      </p>
      <DetailList
        items={[
          { label: 'Key', value: item.key },
          { label: 'Status', value: item.status },
          { label: 'Active', value: isActive ? 'yes' : 'no' },
          {
            label: 'Price',
            value: item.prices?.length
              ? item.prices
                  .map(
                    (p) =>
                      `${(p.unit_amount / 100).toFixed(2)} ${p.currency.toUpperCase()}/${p.interval}${
                        p.synced ? '' : ' (not synced)'
                      }`,
                  )
                  .join(', ')
              : 'none',
          },
          {
            label: 'Features',
            value: item.feature_keys.length ? item.feature_keys.join(', ') : 'none',
          },
        ]}
      />
    </section>
  )
}
