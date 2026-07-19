import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  getProductPlan,
  listProductFeatures,
  setProductPlanFeatures,
  updateProductPlan,
  type ProductFeature,
} from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function ProductPlansEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [status, setStatus] = useState('active')
  const [features, setFeatures] = useState<ProductFeature[]>([])
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void (async () => {
      try {
        const [plan, catalog] = await Promise.all([getProductPlan(id), listProductFeatures(orgId)])
        setName(plan.name)
        setStatus(plan.status)
        setFeatures(catalog.items)
        const sel: Record<string, boolean> = {}
        for (const f of catalog.items) {
          sel[f.key] = plan.feature_keys.includes(f.key)
        }
        setSelected(sel)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      } finally {
        setLoading(false)
      }
    })()
  }, [id, orgId])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateProductPlan(id, { name, status })
      const keys = Object.entries(selected)
        .filter(([, on]) => on)
        .map(([key]) => key)
      await setProductPlanFeatures(id, keys)
      navigate(resourcePath(orgId, 'product-plans', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit product plan" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="active">active</option>
            <option value="archived">archived</option>
          </select>
        </label>
        <fieldset>
          <legend>Product features</legend>
          {features.length === 0 ? (
            <p className="status">Create product features first, then attach them here.</p>
          ) : (
            features.map((f) => (
              <label key={f.id} className="perm">
                <input
                  type="checkbox"
                  checked={!!selected[f.key]}
                  onChange={(e) => setSelected((prev) => ({ ...prev, [f.key]: e.target.checked }))}
                />
                <span>
                  <code>{f.key}</code>
                  {f.description ? ` — ${f.description}` : ''}
                </span>
              </label>
            ))
          )}
        </fieldset>
        <FormActions cancelTo={resourcePath(orgId, 'product-plans', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
