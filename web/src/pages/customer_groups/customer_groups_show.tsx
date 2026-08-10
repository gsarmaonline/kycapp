import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  addAppGroupMember,
  getAppUserGroup,
  listAppGroupMembers,
  listAppUsers,
  removeAppGroupMember,
  type AppGroupMember,
  type AppUser,
} from '../../api'
import { DetailList, PageHeader, ResourceTable } from '../../crud/ui'

/** Membership is managed here, on the group, rather than from a grants screen. */
export function CustomerGroupsShow() {
  const { orgId = '', id = '' } = useParams()
  const [group, setGroup] = useState<{ key: string; name: string; description: string } | null>(null)
  const [members, setMembers] = useState<AppGroupMember[]>([])
  const [customers, setCustomers] = useState<AppUser[]>([])
  const [addUser, setAddUser] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function refresh() {
    setError(null)
    try {
      const [g, m, u] = await Promise.all([
        getAppUserGroup(orgId, id),
        listAppGroupMembers(orgId, id),
        listAppUsers(orgId),
      ])
      setGroup(g)
      setMembers(m.items)
      setCustomers(u.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Not found')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId, id])

  if (error) return <p className="error">{error}</p>
  if (!group) return <p>Loading…</p>

  const inGroup = new Set(members.map((m) => m.id))

  return (
    <section>
      <PageHeader title="Group" />
      <DetailList
        items={[
          { label: 'Name', value: group.name },
          { label: 'Key', value: group.key },
          { label: 'Description', value: group.description || '—' },
          { label: 'Members', value: String(members.length) },
        ]}
      />

      <h3 className="section-title">Members</h3>
      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault()
          void (async () => {
            try {
              await addAppGroupMember(orgId, id, addUser)
              setAddUser('')
              await refresh()
            } catch (err) {
              setError(err instanceof Error ? err.message : 'Add failed')
            }
          })()
        }}
      >
        <select value={addUser} onChange={(e) => setAddUser(e.target.value)} aria-label="Customer" required>
          <option value="">Choose a customer…</option>
          {customers
            .filter((u) => !inGroup.has(u.id))
            .map((u) => (
              <option key={u.id} value={u.id}>
                {u.email ?? u.display_name ?? u.id}
              </option>
            ))}
        </select>
        <button type="submit">Add to group</button>
      </form>

      <ResourceTable
        columns={['Customer', 'Status']}
        empty="No members yet."
        rows={members.map((m) => ({
          key: m.id,
          cells: [m.email ?? m.display_name ?? m.id, m.status],
          actions: (
            <button
              type="button"
              className="link-btn danger"
              onClick={() =>
                void (async () => {
                  await removeAppGroupMember(orgId, id, m.id)
                  await refresh()
                })()
              }
            >
              Remove
            </button>
          ),
        }))}
      />
    </section>
  )
}
