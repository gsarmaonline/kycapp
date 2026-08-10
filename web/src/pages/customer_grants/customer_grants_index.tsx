import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAppGrant, listAppGrants, type AppGrant } from '../../api'
import { PageHeader, ResourceTable } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerGrantsIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AppGrant[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAppGrants(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load grants')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onRevoke(g: AppGrant) {
    if (!confirm(`Revoke ${g.role_key} on ${g.scope_kind}/${g.scope_id} from ${g.subject_label}?`))
      return
    try {
      await deleteAppGrant(orgId, g.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Revoke failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Grants"
        createTo={resourcePath(orgId, 'customer-grants', 'new')}
        createLabel="Grant access"
      />
      <p className="muted">
        Each row gives one subject one role over one scope. Grants are never edited in place: revoke
        and issue a new one, so the history stays readable.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Subject', 'Type', 'Role', 'Scope', 'Expires']}
          empty="No grants yet."
          rows={items.map((g) => ({
            key: g.id,
            cells: [
              g.subject_label,
              g.subject_kind === 'group' ? 'Group' : 'Customer',
              g.role_key,
              `${g.scope_kind} / ${g.scope_id}`,
              g.expires_at ? new Date(g.expires_at).toLocaleDateString() : 'never',
            ],
            actions: (
              <button type="button" className="link-btn danger" onClick={() => void onRevoke(g)}>
                Revoke
              </button>
            ),
          }))}
        />
      )}
    </section>
  )
}
