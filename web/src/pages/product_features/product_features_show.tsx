import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import {
  getProductFeature,
  setProductFeatureOverrides,
  type ProductFeature,
  type ProductFeatureOverride,
} from '../../api'
import { EntitlementUsageChart } from '../../components/EntitlementUsageChart'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function ProductFeaturesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<ProductFeature | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [subjectId, setSubjectId] = useState('')
  const [effect, setEffect] = useState<'include' | 'exclude'>('include')
  const [saving, setSaving] = useState(false)

  async function refresh() {
    setItem(await getProductFeature(id))
  }

  useEffect(() => {
    void refresh().catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
  }, [id])

  async function persistOverrides(next: ProductFeatureOverride[]) {
    setSaving(true)
    setError(null)
    try {
      setItem(await setProductFeatureOverrides(id, { overrides: next }))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update overrides')
    } finally {
      setSaving(false)
    }
  }

  async function onAddOverride(e: FormEvent) {
    e.preventDefault()
    if (!item) return
    const sid = subjectId.trim()
    if (!sid) return
    const rest = item.overrides.filter((o) => o.subject_id !== sid)
    await persistOverrides([...rest, { subject_id: sid, effect }])
    setSubjectId('')
  }

  async function onRemoveOverride(subject: string) {
    if (!item) return
    await persistOverrides(item.overrides.filter((o) => o.subject_id !== subject))
  }

  if (error && !item) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title={item.key} />
      {error && <p className="error">{error}</p>}
      <DetailList
        items={[
          { label: 'Key', value: item.key },
          { label: 'Description', value: item.description || '—' },
          { label: 'Scope', value: item.scope },
          { label: 'Enabled', value: item.enabled ? 'On' : 'Off' },
          { label: 'Rollout', value: `${item.rollout_percentage}%` },
        ]}
        editTo={resourcePath(orgId, 'product-features', item.id, 'edit')}
        backTo={resourcePath(orgId, 'product-features')}
      />

      <h3 style={{ marginTop: '1.5rem' }}>Subject overrides</h3>
      <p className="lede">Force include or exclude specific subjects regardless of percentage.</p>
      {item.overrides.length === 0 ? (
        <p>No overrides.</p>
      ) : (
        <ul>
          {item.overrides.map((o) => (
            <li key={o.subject_id} className="row" style={{ marginBottom: '0.35rem' }}>
              <code>{o.subject_id}</code>
              <span>{o.effect}</span>
              <button
                type="button"
                className="button ghost"
                disabled={saving}
                onClick={() => void onRemoveOverride(o.subject_id)}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
      <form className="create stacked" onSubmit={(e) => void onAddOverride(e)} style={{ marginTop: '1rem' }}>
        <label>
          Subject id
          <input
            value={subjectId}
            onChange={(e) => setSubjectId(e.target.value)}
            placeholder="user_123"
            required
          />
        </label>
        <label>
          Effect
          <select value={effect} onChange={(e) => setEffect(e.target.value as 'include' | 'exclude')}>
            <option value="include">include</option>
            <option value="exclude">exclude</option>
          </select>
        </label>
        <button type="submit" className="button" disabled={saving}>
          Add override
        </button>
      </form>

      <div className="obs-show-block">
        <EntitlementUsageChart
          orgId={orgId}
          entitlementKey={item.key}
          days={14}
          title={`Checks for ${item.key}`}
        />
      </div>
    </section>
  )
}
