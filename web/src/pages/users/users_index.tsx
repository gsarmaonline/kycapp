import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAppUser, listAppUsers, type AppUser } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function UsersIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AppUser[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAppUsers(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load users')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(u: AppUser) {
    if (!confirm(`Archive user ${u.display_name || u.email || u.id}?`)) return
    try {
      await deleteAppUser(u.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Users"
        createTo={resourcePath(orgId, 'users', 'new')}
        createLabel="Create user"
      />
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Email', 'Status', 'Attributes']}
          empty="No users yet."
          rows={items.map((u) => ({
            key: u.id,
            cells: [
              u.display_name || '—',
              u.email || '—',
              u.status,
              String(Object.keys(u.attributes || {}).length),
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'users', u.id)}
                editTo={resourcePath(orgId, 'users', u.id, 'edit')}
                onDelete={() => void onDelete(u)}
                deleteDisabled={u.status === 'archived'}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
