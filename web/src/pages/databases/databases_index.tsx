import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteOrgDatabase, listOrgDatabases, type OrgDatabase } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function DatabasesIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<OrgDatabase[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listOrgDatabases(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load databases')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(d: OrgDatabase) {
    if (!confirm(`Delete database ${d.name}? Automations that use it will fail until updated.`)) {
      return
    }
    try {
      await deleteOrgDatabase(orgId, d.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Databases"
        createTo={resourcePath(orgId, 'databases', 'new')}
        createLabel="Add database"
      />
      <p className="lede">
        Postgres connections for the <code>db_insert</code> automation action.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Host', 'Database', 'User', 'Status', 'Last check']}
          empty="No databases yet."
          rows={items.map((d) => ({
            key: d.id,
            cells: [
              d.name,
              `${d.host}:${d.port}`,
              d.database_name,
              d.username,
              d.status === 'unreachable' && d.last_error
                ? `${d.status}: ${d.last_error}`
                : d.status,
              d.last_checked_at ? new Date(d.last_checked_at).toLocaleString() : '—',
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'databases', d.id)}
                editTo={resourcePath(orgId, 'databases', d.id, 'edit')}
                onDelete={() => void onDelete(d)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
