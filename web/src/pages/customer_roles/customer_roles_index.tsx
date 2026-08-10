import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAppRole, listAppRoles, type AppRole } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerRolesIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AppRole[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAppRoles(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load roles')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(r: AppRole) {
    if (!confirm(`Delete role ${r.name}?`)) return
    try {
      await deleteAppRole(orgId, r.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Roles"
        createTo={resourcePath(orgId, 'customer-roles', 'new')}
        createLabel="Create role"
      />
      <p className="muted">
        Named sets of capabilities. A role may build on others, and editing it reaches everyone
        holding it.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Key', 'Own', 'Effective']}
          empty="No roles yet."
          rows={items.map((r) => ({
            key: r.id,
            cells: [
              r.name,
              r.key,
              String(r.own_capabilities.length),
              // Effective is what a grant actually carries, and it differs from
              // own whenever the role builds on another.
              String(r.effective_capabilities.length),
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'customer-roles', r.id)}
                editTo={resourcePath(orgId, 'customer-roles', r.id, 'edit')}
                onDelete={() => void onDelete(r)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
