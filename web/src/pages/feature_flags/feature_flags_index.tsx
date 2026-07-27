import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteFeatureFlag, listFeatureFlags, type FeatureFlag } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function FeatureFlagsIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<FeatureFlag[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listFeatureFlags(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load feature flags')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(f: FeatureFlag) {
    if (!confirm(`Delete flag ${f.key}?`)) return
    try {
      await deleteFeatureFlag(f.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Feature flags"
        createTo={resourcePath(orgId, 'feature-flags', 'new')}
        createLabel="Create flag"
      />
      <p className="lede">
        Progressive rollouts for your product. Check flags at runtime with a subject id for sticky
        percentage targeting; use overrides to force include or exclude.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Key', 'Enabled', 'Rollout', 'Description']}
          empty="No feature flags yet."
          rows={items.map((f) => ({
            key: f.id,
            cells: [
              f.key,
              f.enabled ? 'On' : 'Off',
              `${f.rollout_percentage}%`,
              f.description || '—',
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'feature-flags', f.id)}
                editTo={resourcePath(orgId, 'feature-flags', f.id, 'edit')}
                onDelete={() => void onDelete(f)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
