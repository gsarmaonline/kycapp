import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteRole, listRoles, type Role } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function RolesIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<Role[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listRoles(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load roles')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(r: Role) {
    if (!confirm(`Delete role ${r.name}?`)) return
    try {
      await deleteRole(r.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Roles"
        createTo={resourcePath(orgId, 'roles', 'new')}
        createLabel="Create role"
      />
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Key', 'System', 'Permissions']}
          empty="No roles yet."
          rows={items.map((r) => ({
            key: r.id,
            cells: [
              r.name,
              r.key,
              r.is_system ? 'yes' : 'no',
              String(r.permission_keys?.length ?? 0),
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'roles', r.id)}
                editTo={resourcePath(orgId, 'roles', r.id, 'edit')}
                onDelete={() => void onDelete(r)}
                deleteDisabled={!!r.is_system}
                deleteTitle={r.is_system ? 'System roles cannot be deleted' : undefined}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
