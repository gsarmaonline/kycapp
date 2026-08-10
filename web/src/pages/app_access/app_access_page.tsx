import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  addAppGroupMember,
  createAppCapability,
  createAppGrant,
  createAppRole,
  createAppScopeType,
  createAppUserGroup,
  deleteAppCapability,
  deleteAppGrant,
  deleteAppRole,
  deleteAppScopeType,
  deleteAppUserGroup,
  listAppCapabilities,
  listAppGrants,
  listAppGroupMembers,
  listAppRoles,
  listAppScopeTypes,
  listAppUserGroups,
  listAppUsers,
  listGroupsForAppUser,
  getAppUserAccess,
  removeAppGroupMember,
  updateAppUserGroup,
  type AppCapability,
  type AppGrant,
  type AppGroupMember,
  type AppRole,
  type AppScopeType,
  type AppUser,
  type AppUserGroup,
  type AppAccessSet,
} from '../../api'
import { ResourceTable } from '../../crud/ui'

type Tab = 'roles' | 'groups' | 'grants' | 'vocabulary' | 'customer'

const TABS: { id: Tab; label: string; hint: string }[] = [
  { id: 'vocabulary', label: 'Vocabulary', hint: 'The scope kinds and capabilities your product uses' },
  { id: 'roles', label: 'Roles', hint: 'Named sets of capabilities, which may build on each other' },
  { id: 'groups', label: 'Groups', hint: 'Sets of customers a role can be granted to at once' },
  { id: 'grants', label: 'Grants', hint: 'Who holds which role, and where' },
  {
    id: 'customer',
    label: 'Customer',
    hint: 'Why one customer has the access they have',
  },
]

export function AppAccessPage() {
  const { orgId = '' } = useParams()
  const [tab, setTab] = useState<Tab>('vocabulary')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const [scopeTypes, setScopeTypes] = useState<AppScopeType[]>([])
  const [capabilities, setCapabilities] = useState<AppCapability[]>([])
  const [roles, setRoles] = useState<AppRole[]>([])
  const [groups, setGroups] = useState<AppUserGroup[]>([])
  const [grants, setGrants] = useState<AppGrant[]>([])
  const [appUsers, setAppUsers] = useState<AppUser[]>([])

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const [s, c, r, g, gr, u] = await Promise.all([
        listAppScopeTypes(orgId),
        listAppCapabilities(orgId),
        listAppRoles(orgId),
        listAppUserGroups(orgId),
        listAppGrants(orgId),
        listAppUsers(orgId),
      ])
      setScopeTypes(s.items)
      setCapabilities(c.items)
      setRoles(r.items)
      setGroups(g.items)
      setGrants(gr.items)
      setAppUsers(u.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load access configuration')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function run(fn: () => Promise<unknown>) {
    setError(null)
    try {
      await fn()
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Request failed')
    }
  }

  return (
    <section>
      <header className="page-header">
        <div>
          <h1>Customer access</h1>
          <p className="muted">
            Define what your customers may do inside your product. Configure it here, enforce it in
            your backend with the grant set from <code>GET /v1/app-users/&#123;id&#125;/access</code>.
          </p>
        </div>
      </header>

      <nav className="row tabs" aria-label="Customer access sections">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            className={t.id === tab ? 'tab tab-active' : 'tab'}
            aria-current={t.id === tab ? 'page' : undefined}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </nav>
      <p className="muted">{TABS.find((t) => t.id === tab)?.hint}</p>

      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <>
          {tab === 'vocabulary' && (
            <VocabularyTab
              orgId={orgId}
              scopeTypes={scopeTypes}
              capabilities={capabilities}
              run={run}
            />
          )}
          {tab === 'roles' && (
            <RolesTab orgId={orgId} roles={roles} capabilities={capabilities} run={run} />
          )}
          {tab === 'groups' && (
            <GroupsTab orgId={orgId} groups={groups} appUsers={appUsers} run={run} />
          )}
          {tab === 'customer' && <CustomerTab appUsers={appUsers} />}
          {tab === 'grants' && (
            <GrantsTab
              orgId={orgId}
              grants={grants}
              roles={roles}
              groups={groups}
              appUsers={appUsers}
              scopeTypes={scopeTypes}
              run={run}
            />
          )}
        </>
      )}
    </section>
  )
}

type Runner = (fn: () => Promise<unknown>) => Promise<void>

function VocabularyTab({
  orgId,
  scopeTypes,
  capabilities,
  run,
}: {
  orgId: string
  scopeTypes: AppScopeType[]
  capabilities: AppCapability[]
  run: Runner
}) {
  const [kind, setKind] = useState('')
  const [capKey, setCapKey] = useState('')

  return (
    <div className="stack">
      <div>
        <h2>Scope kinds</h2>
        <p className="muted">
          The levels your product has, such as <code>project</code> or <code>environment</code>. You
          declare the kind; the ids stay in your system and are never registered here.
        </p>
        <form
          className="row"
          onSubmit={(e) => {
            e.preventDefault()
            void run(async () => {
              await createAppScopeType(orgId, { kind })
              setKind('')
            })
          }}
        >
          <input
            value={kind}
            onChange={(e) => setKind(e.target.value)}
            placeholder="project"
            aria-label="Scope kind"
            required
          />
          <button type="submit">Add scope kind</button>
        </form>
        <ResourceTable
          columns={['Kind', 'Label']}
          empty="No scope kinds yet. Add one before granting."
          rows={scopeTypes.map((s) => ({
            key: s.id,
            cells: [<code key="k">{s.kind}</code>, s.label || '—'],
            actions: (
              <button type="button" onClick={() => void run(() => deleteAppScopeType(orgId, s.id))}>
                Delete
              </button>
            ),
          }))}
        />
      </div>

      <div>
        <h2>Capabilities</h2>
        <p className="muted">
          The verbs your product checks, as <code>resource:action</code>. A role can only use
          capabilities declared here, so a typo is caught rather than silently granting nothing.
        </p>
        <form
          className="row"
          onSubmit={(e) => {
            e.preventDefault()
            void run(async () => {
              await createAppCapability(orgId, { key: capKey })
              setCapKey('')
            })
          }}
        >
          <input
            value={capKey}
            onChange={(e) => setCapKey(e.target.value)}
            placeholder="deploy:production"
            aria-label="Capability key"
            required
          />
          <button type="submit">Add capability</button>
        </form>
        <ResourceTable
          columns={['Key', 'Description']}
          empty="No capabilities yet."
          rows={capabilities.map((c) => ({
            key: c.id,
            cells: [<code key="k">{c.key}</code>, c.description || '—'],
            actions: (
              <button type="button" onClick={() => void run(() => deleteAppCapability(orgId, c.id))}>
                Delete
              </button>
            ),
          }))}
        />
      </div>
    </div>
  )
}

function RolesTab({
  orgId,
  roles,
  capabilities,
  run,
}: {
  orgId: string
  roles: AppRole[]
  capabilities: AppCapability[]
  run: Runner
}) {
  const [key, setKey] = useState('')
  const [picked, setPicked] = useState<string[]>([])
  const [extendsIds, setExtendsIds] = useState<string[]>([])

  const roleById = useMemo(() => new Map(roles.map((r) => [r.id, r])), [roles])

  return (
    <div className="stack">
      <form
        className="stack"
        onSubmit={(e) => {
          e.preventDefault()
          void run(async () => {
            await createAppRole(orgId, { key, capabilities: picked, extends: extendsIds })
            setKey('')
            setPicked([])
            setExtendsIds([])
          })
        }}
      >
        <div className="row">
          <input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="maintainer"
            aria-label="Role key"
            required
          />
          <button type="submit">Create role</button>
        </div>
        <fieldset>
          <legend>Capabilities this role adds</legend>
          {capabilities.length === 0 ? (
            <p className="muted">Declare a capability first.</p>
          ) : (
            <div className="row wrap">
              {capabilities.map((c) => (
                <label key={c.id}>
                  <input
                    type="checkbox"
                    checked={picked.includes(c.key)}
                    onChange={(e) =>
                      setPicked((prev) =>
                        e.target.checked ? [...prev, c.key] : prev.filter((k) => k !== c.key),
                      )
                    }
                  />{' '}
                  <code>{c.key}</code>
                </label>
              ))}
            </div>
          )}
        </fieldset>
        <fieldset>
          <legend>Builds on</legend>
          <p className="muted">
            Capabilities from these roles are included automatically. Editing them later updates
            everyone holding this role.
          </p>
          {roles.length === 0 ? (
            <p className="muted">No roles to build on yet.</p>
          ) : (
            <div className="row wrap">
              {roles.map((r) => (
                <label key={r.id}>
                  <input
                    type="checkbox"
                    checked={extendsIds.includes(r.id)}
                    onChange={(e) =>
                      setExtendsIds((prev) =>
                        e.target.checked ? [...prev, r.id] : prev.filter((id) => id !== r.id),
                      )
                    }
                  />{' '}
                  {r.name}
                </label>
              ))}
            </div>
          )}
        </fieldset>
      </form>

      <ResourceTable
        columns={['Role', 'Own', 'Effective']}
        empty="No roles yet."
        rows={roles.map((r) => ({
          key: r.id,
          cells: [
            <div key="n">
              <strong>{r.name}</strong>
              <br />
              <code className="muted">{r.key}</code>
            </div>,
            <CapList key="o" items={r.own_capabilities} />,
            // Showing only the chain is how inheritance surprises people, so the
            // resolved set is always displayed next to it.
            <CapList key="e" items={r.effective_capabilities} inheritedFrom={r} lookup={roleById} />,
          ],
          actions: (
            <button type="button" onClick={() => void run(() => deleteAppRole(orgId, r.id))}>
              Delete
            </button>
          ),
        }))}
      />
    </div>
  )
}

function CapList({
  items,
  inheritedFrom,
  lookup,
}: {
  items: string[]
  inheritedFrom?: AppRole
  lookup?: Map<string, AppRole>
}) {
  if (items.length === 0) return <span className="muted">none</span>
  const own = new Set(inheritedFrom?.own_capabilities ?? [])
  return (
    <div className="row wrap">
      {items.map((k) => {
        const inherited = inheritedFrom && lookup && !own.has(k)
        return (
          <code key={k} title={inherited ? 'Inherited' : undefined}>
            {k}
            {inherited ? ' ↑' : ''}
          </code>
        )
      })}
    </div>
  )
}

function GroupsTab({
  orgId,
  groups,
  appUsers,
  run,
}: {
  orgId: string
  groups: AppUserGroup[]
  appUsers: AppUser[]
  run: Runner
}) {
  const [key, setKey] = useState('')
  const [openGroup, setOpenGroup] = useState<string | null>(null)
  const [members, setMembers] = useState<AppGroupMember[]>([])
  const [addUser, setAddUser] = useState('')

  async function openMembers(groupId: string) {
    setOpenGroup(groupId)
    setMembers((await listAppGroupMembers(orgId, groupId)).items)
  }

  return (
    <div className="stack">
      <p className="muted">
        A grant to a group reaches every member. Membership is an explicit list; rules over
        attributes are not supported yet.
      </p>
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
          <h2>Members</h2>
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
            <select value={addUser} onChange={(e) => setAddUser(e.target.value)} aria-label="Customer" required>
              <option value="">Choose a customer…</option>
              {appUsers.map((u) => (
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
    </div>
  )
}

function GrantsTab({
  orgId,
  grants,
  roles,
  groups,
  appUsers,
  scopeTypes,
  run,
}: {
  orgId: string
  grants: AppGrant[]
  roles: AppRole[]
  groups: AppUserGroup[]
  appUsers: AppUser[]
  scopeTypes: AppScopeType[]
  run: Runner
}) {
  const [subjectKind, setSubjectKind] = useState<'app_user' | 'group'>('app_user')
  const [subjectId, setSubjectId] = useState('')
  const [roleId, setRoleId] = useState('')
  const [scopeKind, setScopeKind] = useState('')
  const [scopeId, setScopeId] = useState('')

  const ready = roles.length > 0 && scopeTypes.length > 0

  return (
    <div className="stack">
      {!ready && (
        <p className="muted">Declare a scope kind and create a role before granting.</p>
      )}
      <form
        className="row wrap"
        onSubmit={(e) => {
          e.preventDefault()
          void run(async () => {
            await createAppGrant(orgId, {
              ...(subjectKind === 'group' ? { group_id: subjectId } : { app_user_id: subjectId }),
              role_id: roleId,
              scope_kind: scopeKind,
              scope_id: scopeId,
            })
            setSubjectId('')
            setScopeId('')
          })
        }}
      >
        <select
          value={subjectKind}
          onChange={(e) => {
            setSubjectKind(e.target.value as 'app_user' | 'group')
            setSubjectId('')
          }}
          aria-label="Grant to"
        >
          <option value="app_user">A customer</option>
          <option value="group">A group</option>
        </select>
        <select value={subjectId} onChange={(e) => setSubjectId(e.target.value)} aria-label="Subject" required>
          <option value="">Choose…</option>
          {subjectKind === 'group'
            ? groups.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))
            : appUsers.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.email ?? u.display_name ?? u.id}
                </option>
              ))}
        </select>
        <select value={roleId} onChange={(e) => setRoleId(e.target.value)} aria-label="Role" required>
          <option value="">Role…</option>
          {roles.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>
        <select value={scopeKind} onChange={(e) => setScopeKind(e.target.value)} aria-label="Scope kind" required>
          <option value="">Scope kind…</option>
          {scopeTypes.map((s) => (
            <option key={s.id} value={s.kind}>
              {s.kind}
            </option>
          ))}
        </select>
        <input
          value={scopeId}
          onChange={(e) => setScopeId(e.target.value)}
          placeholder="your id, e.g. p1"
          aria-label="Scope id"
          required
        />
        <button type="submit" disabled={!ready}>
          Grant
        </button>
      </form>

      <ResourceTable
        columns={['Subject', 'Role', 'Scope']}
        empty="No grants yet."
        rows={grants.map((g) => ({
          key: g.id,
          cells: [
            <span key="s">
              {g.subject_kind === 'group' ? 'Group ' : ''}
              <strong>{g.subject_label || '—'}</strong>
            </span>,
            <code key="r">{g.role_key}</code>,
            <code key="sc">
              {g.scope_kind}:{g.scope_id}
            </code>,
          ],
          actions: (
            <button type="button" onClick={() => void run(() => deleteAppGrant(orgId, g.id))}>
              Revoke
            </button>
          ),
        }))}
      />
    </div>
  )
}

/**
 * An editable group name. Renaming is separate from the key, which stays fixed
 * because grants and any of the merchant's own tooling refer to it.
 */
function GroupName({
  orgId,
  group,
  run,
}: {
  orgId: string
  group: AppUserGroup
  run: Runner
}) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(group.name)

  if (!editing) {
    return (
      <div>
        <strong>{group.name}</strong>{' '}
        <button type="button" className="link" onClick={() => setEditing(true)}>
          Rename
        </button>
        {/* The key repeats the name until one is set, so only show it when it differs. */}
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

/**
 * Answers "why does this customer have this access?".
 *
 * The group list and the resolved grant set together are the answer: the groups
 * they belong to, and every capability they end up with, each labelled with
 * where it came from.
 */
function CustomerTab({ appUsers }: { appUsers: AppUser[] }) {
  const [selected, setSelected] = useState('')
  const [groups, setGroups] = useState<{ id: string; key: string; name: string }[]>([])
  const [access, setAccess] = useState<AppAccessSet | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!selected) {
      setGroups([])
      setAccess(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    Promise.all([listGroupsForAppUser(selected), getAppUserAccess(selected)])
      .then(([g, a]) => {
        if (cancelled) return
        setGroups(g.items)
        setAccess(a)
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [selected])

  return (
    <div className="stack">
      <select
        value={selected}
        onChange={(e) => setSelected(e.target.value)}
        aria-label="Customer"
      >
        <option value="">Choose a customer…</option>
        {appUsers.map((u) => (
          <option key={u.id} value={u.id}>
            {u.email ?? u.display_name ?? u.id}
          </option>
        ))}
      </select>

      {error && <p className="error">{error}</p>}
      {loading && <p>Loading…</p>}

      {selected && !loading && (
        <>
          <div>
            <h2>Groups</h2>
            <ResourceTable
              columns={['Group', 'Key']}
              empty="Not in any group."
              rows={groups.map((g) => ({
                key: g.id,
                cells: [g.name, <code key="k">{g.key}</code>],
              }))}
            />
          </div>

          <div>
            <h2>Effective access</h2>
            <p className="muted">
              Exactly what your backend receives from{' '}
              <code>GET /v1/app-users/&#123;id&#125;/access</code>
              {access ? ` — version ${access.version}` : ''}.
            </p>
            <ResourceTable
              columns={['Scope', 'Capabilities', 'Via']}
              empty="No access. Grant a role, directly or through a group."
              rows={(access?.grants ?? []).map((g) => ({
                key: g.id,
                cells: [
                  <code key="s">
                    {g.scope_kind}:{g.scope_id}
                  </code>,
                  <div className="row wrap" key="c">
                    {g.capabilities.map((c) => (
                      <code key={c}>{c}</code>
                    ))}
                  </div>,
                  // Provenance is the whole point of this view: a capability
                  // held through a group says so, rather than appearing from
                  // nowhere.
                  <span key="v" className="muted">
                    {g.source}
                  </span>,
                ],
              }))}
            />
          </div>
        </>
      )}
    </div>
  )
}
