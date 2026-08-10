import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAppUserGroup, listAppUserGroups, type AppUserGroup } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerGroupsIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AppUserGroup[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAppUserGroups(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load groups')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(g: AppUserGroup) {
    if (!confirm(`Delete group ${g.name}?`)) return
    try {
      await deleteAppUserGroup(orgId, g.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Groups"
        createTo={resourcePath(orgId, 'customer-groups', 'new')}
        createLabel="Create group"
      />
      <p className="muted">
        Sets of your customers. Granting a role to a group reaches every member. Membership is an
        explicit list, managed on the group.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Key', 'Members']}
          empty="No groups yet."
          rows={items.map((g) => ({
            key: g.id,
            cells: [g.name, g.key, String(g.member_count)],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'customer-groups', g.id)}
                editTo={resourcePath(orgId, 'customer-groups', g.id, 'edit')}
                onDelete={() => void onDelete(g)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
