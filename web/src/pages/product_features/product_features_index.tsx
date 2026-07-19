import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteProductFeature, listProductFeatures, type ProductFeature } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function ProductFeaturesIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<ProductFeature[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listProductFeatures(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load product features')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(f: ProductFeature) {
    if (!confirm(`Delete feature ${f.key}?`)) return
    try {
      await deleteProductFeature(f.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Product features"
        createTo={resourcePath(orgId, 'product-features', 'new')}
        createLabel="Create feature"
      />
      <p className="lede">
        Named capabilities your product can gate for end users. Package them into plans, then activate a
        plan on Billing.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Key', 'Description']}
          empty="No product features yet."
          rows={items.map((f) => ({
            key: f.id,
            cells: [f.key, f.description || '—'],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'product-features', f.id)}
                editTo={resourcePath(orgId, 'product-features', f.id, 'edit')}
                onDelete={() => void onDelete(f)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
