import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAppScopeType, listAppScopeTypes, type AppScopeType } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerScopeKindsIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AppScopeType[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAppScopeTypes(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load scope kinds')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(s: AppScopeType) {
    if (!confirm(`Delete scope kind ${s.kind}?`)) return
    try {
      await deleteAppScopeType(orgId, s.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Scope kinds"
        createTo={resourcePath(orgId, 'customer-scope-kinds', 'new')}
        createLabel="Add scope kind"
      />
      <p className="muted">
        The levels your product has, such as <code>project</code>. You declare the kind; the ids stay
        in your system and are never registered here.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Kind', 'Label']}
          empty="No scope kinds yet."
          rows={items.map((s) => ({
            key: s.id,
            cells: [s.kind, s.label || '—'],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'customer-scope-kinds', s.id)}
                editTo={resourcePath(orgId, 'customer-scope-kinds', s.id, 'edit')}
                onDelete={() => void onDelete(s)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
