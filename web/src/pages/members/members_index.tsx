import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteMembership,
  listMemberships,
  type Membership,
} from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function MembersIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<Membership[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const res = await listMemberships(orgId)
      setItems(res.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load members')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(m: Membership) {
    if (!confirm(`Remove ${m.user_email || m.user_name || 'member'}?`)) return
    try {
      await deleteMembership(m.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Members"
        createTo={resourcePath(orgId, 'members', 'new')}
        createLabel="Create member"
      />
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Email', 'Role', 'Status']}
          empty="No members yet."
          rows={items.map((m) => ({
            key: m.id,
            cells: [
              m.user_name || '—',
              m.user_email || '—',
              m.role_key || '—',
              <span className="status" key="s">
                {m.status}
              </span>,
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'members', m.id)}
                editTo={resourcePath(orgId, 'members', m.id, 'edit')}
                onDelete={() => void onDelete(m)}
                deleteDisabled={m.status === 'revoked'}
                deleteTitle={m.status === 'revoked' ? 'Already revoked' : undefined}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
