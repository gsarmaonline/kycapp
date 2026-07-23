import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteProductPlan,
  getActiveProductPlan,
  listProductPlans,
  type ProductPlan,
} from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function ProductPlansIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<ProductPlan[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const plans = await listProductPlans(orgId)
      setItems(plans.items)
      try {
        const active = await getActiveProductPlan(orgId)
        setActiveId(active.id)
      } catch {
        setActiveId(null)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load product plans')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(p: ProductPlan) {
    if (!confirm(`Delete plan ${p.name}?`)) return
    try {
      await deleteProductPlan(p.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Product plans"
        createTo={resourcePath(orgId, 'product-plans', 'new')}
        createLabel="Create plan"
      />
      <p className="lede">
        Packages of product features with optional Stripe prices. Activate one plan to control what
        your product users may use (checked via entitlements).
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Key', 'Price', 'Status', 'Features', 'Active']}
          empty="No product plans yet."
          rows={items.map((p) => ({
            key: p.id,
            cells: [
              p.name,
              p.key,
              p.prices?.length
                ? p.prices
                    .map(
                      (pr) =>
                        `${(pr.unit_amount / 100).toFixed(2)} ${pr.currency.toUpperCase()}/${pr.interval}`,
                    )
                    .join(', ')
                : '—',
              p.status,
              p.feature_keys.length ? p.feature_keys.join(', ') : '—',
              p.id === activeId ? 'yes' : '—',
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'product-plans', p.id)}
                editTo={resourcePath(orgId, 'product-plans', p.id, 'edit')}
                onDelete={() => void onDelete(p)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
