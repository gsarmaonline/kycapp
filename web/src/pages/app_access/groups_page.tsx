import { useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  addAppGroupMember,
  createAppUserGroup,
  deleteAppUserGroup,
  listAppGroupMembers,
  listAppUserGroups,
  listAppUsers,
  removeAppGroupMember,
  updateAppUserGroup,
  type AppGroupMember,
  type AppUserGroup,
} from '../../api'
import { ResourceTable } from '../../crud/ui'
import { AccessHeader, useResource, type Runner } from './shared'

export function CustomerGroupsPage() {
  const { orgId = '' } = useParams()
  const { data, error, loading, run } = useResource(
    async () => ({
      groups: (await listAppUserGroups(orgId)).items,
      customers: (await listAppUsers(orgId)).items,
    }),
    [orgId],
  )
  const [key, setKey] = useState('')
  const [openGroup, setOpenGroup] = useState<string | null>(null)
  const [members, setMembers] = useState<AppGroupMember[]>([])
  const [addUser, setAddUser] = useState('')

  const groups = data?.groups ?? []
  const customers = data?.customers ?? []

  async function openMembers(groupId: string) {
    setOpenGroup(groupId)
    setMembers((await listAppGroupMembers(orgId, groupId)).items)
  }

  return (
    <section>
      <AccessHeader title="Groups">
        Sets of your customers. Granting a role to a group reaches every member, so a set is
        configured once instead of per customer. Membership is an explicit list.
      </AccessHeader>

      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault()
          void run(async () => {
            await createAppUserGroup(orgId, { key })
            setKey('')
          })
        }}
      >
        <input
          value={key}
          onChange={(e) => setKey(e.target.value)}
          placeholder="enterprise_customers"
          aria-label="Group key"
          required
        />
        <button type="submit">Create group</button>
      </form>

      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <>
          <ResourceTable
            columns={['Group', 'Members']}
            empty="No groups yet."
            rows={groups.map((g) => ({
              key: g.id,
              cells: [<GroupName key="n" orgId={orgId} group={g} run={run} />, String(g.member_count)],
              actions: (
                <>
                  <button type="button" onClick={() => void openMembers(g.id)}>
                    Members
                  </button>
                  <button type="button" onClick={() => void run(() => deleteAppUserGroup(orgId, g.id))}>
                    Delete
                  </button>
                </>
              ),
            }))}
          />

          {openGroup && (
            <div className="stack">
              <h2>Members of {groups.find((g) => g.id === openGroup)?.name ?? 'group'}</h2>
              <form
                className="row"
                onSubmit={(e) => {
                  e.preventDefault()
                  void run(async () => {
                    await addAppGroupMember(orgId, openGroup, addUser)
                    setAddUser('')
                    await openMembers(openGroup)
                  })
                }}
              >
                <select
                  value={addUser}
                  onChange={(e) => setAddUser(e.target.value)}
                  aria-label="Customer"
                  required
                >
                  <option value="">Choose a customer…</option>
                  {customers.map((u) => (
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
                      onClick={() =>
                        void run(async () => {
                          await removeAppGroupMember(orgId, openGroup, m.id)
                          await openMembers(openGroup)
                        })
                      }
                    >
                      Remove
                    </button>
                  ),
                }))}
              />
            </div>
          )}
        </>
      )}
    </section>
  )
}

/**
 * An editable group name. The key stays fixed because grants and your own
 * tooling refer to it; only the display name changes.
 */
function GroupName({ orgId, group, run }: { orgId: string; group: AppUserGroup; run: Runner }) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(group.name)

  if (!editing) {
    return (
      <div>
        <strong>{group.name}</strong>{' '}
        <button type="button" className="link" onClick={() => setEditing(true)}>
          Rename
        </button>
        {group.key !== group.name && (
          <>
            <br />
            <code className="muted">{group.key}</code>
          </>
        )}
      </div>
    )
  }
  return (
    <form
      className="row"
      onSubmit={(e) => {
        e.preventDefault()
        void run(async () => {
          await updateAppUserGroup(orgId, group.id, { name })
          setEditing(false)
        })
      }}
    >
      <input value={name} onChange={(e) => setName(e.target.value)} aria-label="Group name" required />
      <button type="submit">Save</button>
      <button
        type="button"
        onClick={() => {
          setName(group.name)
          setEditing(false)
        }}
      >
        Cancel
      </button>
    </form>
  )
}
