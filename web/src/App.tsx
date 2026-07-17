import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import {
  Navigate,
  NavLink,
  Route,
  Routes,
  useNavigate,
  useParams,
} from 'react-router-dom'
import {
  authProviders,
  captureOAuthTokenFromHash,
  createOrganisation,
  createRole,
  devLogin,
  getOrganisation,
  getOrgEntitlements,
  getSubscription,
  getToken,
  googleAuthURL,
  inviteMember,
  listEntitlementsCatalog,
  listMemberships,
  listOrganisations,
  listPermissions,
  listPlans,
  listRoles,
  logout,
  me,
  setToken,
  updateRole,
  type AuthProviders,
  type Membership,
  type Organisation,
  type Permission,
  type Plan,
  type Role,
  type Subscription,
  type User,
} from './api'
import './App.css'

type Gate = 'loading' | 'auth' | 'app'
type OrgSection = 'overview' | 'members' | 'roles' | 'billing'

const ORG_SECTIONS: { id: OrgSection; label: string; path: string }[] = [
  { id: 'overview', label: 'Overview', path: '' },
  { id: 'members', label: 'Members', path: 'members' },
  { id: 'roles', label: 'Roles', path: 'roles' },
  { id: 'billing', label: 'Billing', path: 'billing' },
]

function sectionFromParam(section?: string): OrgSection {
  switch (section) {
    case 'members':
    case 'roles':
    case 'billing':
      return section
    default:
      return 'overview'
  }
}

function orgPath(orgId: string, section: OrgSection = 'overview') {
  const base = `/orgs/${orgId}`
  if (section === 'overview') return base
  return `${base}/${section}`
}

export default function App() {
  const [gate, setGate] = useState<Gate>('loading')
  const [user, setUser] = useState<User | null>(null)
  const navigate = useNavigate()

  async function refreshSession() {
    captureOAuthTokenFromHash()
    const token = getToken()
    if (!token) {
      setUser(null)
      setGate('auth')
      return
    }
    try {
      const res = await me()
      setUser(res.user)
      setGate('app')
    } catch {
      setToken(null)
      setUser(null)
      setGate('auth')
    }
  }

  useEffect(() => {
    void refreshSession()
  }, [])

  async function onAuthed(token: string) {
    setToken(token)
    await refreshSession()
    navigate('/orgs', { replace: true })
  }

  async function onLogout() {
    await logout()
    setUser(null)
    setGate('auth')
    navigate('/', { replace: true })
  }

  if (gate === 'loading') {
    return (
      <main className="page">
        <p>Loading…</p>
      </main>
    )
  }

  if (gate === 'auth') {
    return <AuthScreen onAuthed={onAuthed} />
  }

  return (
    <Routes>
      <Route path="/" element={<Navigate to="/orgs" replace />} />
      <Route path="/orgs" element={<AppShell user={user} onLogout={onLogout} />} />
      <Route path="/orgs/:orgId" element={<AppShell user={user} onLogout={onLogout} />} />
      <Route path="/orgs/:orgId/:section" element={<AppShell user={user} onLogout={onLogout} />} />
      <Route path="*" element={<Navigate to="/orgs" replace />} />
    </Routes>
  )
}

function AuthScreen({ onAuthed }: { onAuthed: (token: string) => Promise<void> }) {
  const [providers, setProviders] = useState<AuthProviders | null>(null)
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void authProviders()
      .then(setProviders)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load auth'))
  }, [])

  async function onDevLogin(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const res = await devLogin(email, name || email)
      await onAuthed(res.token)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign-in failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="page auth-page">
      <header>
        <p className="eyebrow">KYC</p>
        <h1>Sign in</h1>
        <p className="lede">Continue with Google to manage your organisations.</p>
      </header>

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}

      {providers?.google && (
        <a className="google-btn" href={googleAuthURL()}>
          Continue with Google
        </a>
      )}

      {!providers?.google && !providers?.dev_login && providers !== null && (
        <p className="lede">Google OAuth is not configured on this server.</p>
      )}

      {providers?.dev_login && (
        <form className="auth-form" onSubmit={onDevLogin}>
          <p className="lede">Local/dev sign-in (AUTH_DEV_LOGIN)</p>
          <label>
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
            />
          </label>
          <label>
            Name
            <input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" />
          </label>
          <button type="submit" disabled={busy}>
            {busy ? 'Please wait…' : 'Dev sign in'}
          </button>
        </form>
      )}
    </main>
  )
}

function AppShell({ user, onLogout }: { user: User | null; onLogout: () => Promise<void> }) {
  const { orgId: routeOrgId, section: routeSection } = useParams()
  const navigate = useNavigate()
  const section = sectionFromParam(routeSection)

  const [orgs, setOrgs] = useState<Organisation[]>([])
  const [orgsError, setOrgsError] = useState<string | null>(null)
  const [orgsLoading, setOrgsLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [newOrgName, setNewOrgName] = useState('')

  const selected = orgs.find((o) => o.id === routeOrgId) ?? null

  async function refreshOrgs() {
    setOrgsLoading(true)
    setOrgsError(null)
    try {
      const res = await listOrganisations()
      setOrgs(res.items)
      return res.items
    } catch (e) {
      setOrgsError(e instanceof Error ? e.message : 'Failed to load organisations')
      return [] as Organisation[]
    } finally {
      setOrgsLoading(false)
    }
  }

  useEffect(() => {
    void (async () => {
      const items = await refreshOrgs()
      if (!routeOrgId && items[0]) {
        navigate(orgPath(items[0].id, 'overview'), { replace: true })
        return
      }
      if (routeOrgId && items.length > 0 && !items.some((o) => o.id === routeOrgId)) {
        navigate(items[0] ? orgPath(items[0].id) : '/orgs', { replace: true })
      }
    })()
  }, [routeOrgId])

  function switchOrg(id: string) {
    navigate(orgPath(id, section === 'overview' ? 'overview' : section))
  }

  async function onCreateOrg(e: FormEvent) {
    e.preventDefault()
    setOrgsError(null)
    try {
      const org = await createOrganisation(newOrgName)
      setNewOrgName('')
      setCreating(false)
      await refreshOrgs()
      navigate(orgPath(org.id, 'overview'))
    } catch (err) {
      setOrgsError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <div className="shell">
      <aside className="sidebar" aria-label="Organisation navigation">
        <div className="sidebar-brand">
          <p className="eyebrow">KYC</p>
          <strong>Organisations</strong>
        </div>

        <label className="org-switcher">
          <span>Current organisation</span>
          <select
            value={routeOrgId ?? ''}
            disabled={orgsLoading || orgs.length === 0}
            onChange={(e) => switchOrg(e.target.value)}
            aria-label="Switch organisation"
          >
            {orgs.length === 0 && <option value="">No organisations</option>}
            {orgs.map((o) => (
              <option key={o.id} value={o.id}>
                {o.name}
              </option>
            ))}
          </select>
        </label>

        {routeOrgId && (
          <nav className="sidebar-nav" aria-label="Organisation sections">
            {ORG_SECTIONS.map((item) => (
              <NavLink
                key={item.id}
                to={orgPath(routeOrgId, item.id)}
                className={({ isActive }) => (isActive ? 'nav-item active' : 'nav-item')}
                end={item.id === 'overview'}
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
        )}

        <div className="sidebar-actions">
          <button type="button" className="ghost full" onClick={() => setCreating((v) => !v)}>
            {creating ? 'Cancel' : 'New organisation'}
          </button>
          {creating && (
            <form className="create-org" onSubmit={onCreateOrg}>
              <input
                value={newOrgName}
                onChange={(e) => setNewOrgName(e.target.value)}
                placeholder="Organisation name"
                required
              />
              <button type="submit">Create</button>
            </form>
          )}
        </div>

        <div className="sidebar-footer">
          {user && <p className="sidebar-user">{user.email}</p>}
          <button type="button" className="ghost full" onClick={() => void onLogout()}>
            Sign out
          </button>
        </div>
      </aside>

      <main className="main">
        {orgsError && (
          <p className="error" role="alert">
            {orgsError}
          </p>
        )}
        {orgsLoading && <p>Loading organisations…</p>}
        {!orgsLoading && !selected && (
          <div className="empty-state">
            <h1>Your organisations</h1>
            <p className="lede">Create an organisation to invite teammates and manage access.</p>
          </div>
        )}
        {selected && (
          <OrgWorkspace
            key={selected.id}
            orgId={selected.id}
            section={section}
            user={user}
          />
        )}
      </main>
    </div>
  )
}

function OrgWorkspace({
  orgId,
  section,
  user,
}: {
  orgId: string
  section: OrgSection
  user: User | null
}) {
  const [org, setOrg] = useState<Organisation | null>(null)
  const [members, setMembers] = useState<Membership[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setError(null)
    setLoading(true)
    try {
      const [o, m, r, p] = await Promise.all([
        getOrganisation(orgId),
        listMemberships(orgId),
        listRoles(orgId),
        listPermissions(),
      ])
      setOrg(o)
      setMembers(m.items)
      setRoles(r.items)
      setPermissions(p.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  const title = ORG_SECTIONS.find((s) => s.id === section)?.label ?? 'Overview'

  return (
    <div className="workspace">
      <header className="workspace-header">
        <div>
          <p className="eyebrow">{org?.slug ?? '…'}</p>
          <h1>{org?.name ?? 'Organisation'}</h1>
          <p className="status">
            {org?.status ?? '…'}
            {user ? ` · ${user.email}` : ''}
          </p>
        </div>
        <h2 className="section-title">{title}</h2>
      </header>

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {loading && <p>Loading…</p>}

      {!loading && section === 'overview' && org && (
        <OverviewPanel org={org} memberCount={members.length} roleCount={roles.length} />
      )}
      {!loading && section === 'members' && (
        <MembersPanel
          orgId={orgId}
          members={members}
          roles={roles}
          onChanged={refresh}
          onError={setError}
        />
      )}
      {!loading && section === 'roles' && (
        <RoleEditor
          orgId={orgId}
          roles={roles}
          permissions={permissions}
          onChanged={refresh}
          onError={setError}
        />
      )}
      {!loading && section === 'billing' && <BillingPanel orgId={orgId} onError={setError} />}
    </div>
  )
}

function OverviewPanel({
  org,
  memberCount,
  roleCount,
}: {
  org: Organisation
  memberCount: number
  roleCount: number
}) {
  return (
    <section className="overview">
      <p className="lede">
        Use the sidebar to open Members, Roles, or Billing. Switch organisations with the
        dropdown above.
      </p>
      <ul className="overview-stats">
        <li>
          <strong>{memberCount}</strong>
          <span>Members</span>
        </li>
        <li>
          <strong>{roleCount}</strong>
          <span>Roles</span>
        </li>
        <li>
          <strong>{org.status}</strong>
          <span>Status</span>
        </li>
      </ul>
    </section>
  )
}

function MembersPanel({
  orgId,
  members,
  roles,
  onChanged,
  onError,
}: {
  orgId: string
  members: Membership[]
  roles: Role[]
  onChanged: () => Promise<void>
  onError: (msg: string | null) => void
}) {
  const [email, setEmail] = useState('')
  const [roleId, setRoleId] = useState('')

  useEffect(() => {
    const memberRole = roles.find((x) => x.key === 'member')
    setRoleId((prev) => prev || memberRole?.id || roles[0]?.id || '')
  }, [roles])

  async function onInvite(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      await inviteMember(orgId, email, roleId)
      setEmail('')
      await onChanged()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Invite failed')
    }
  }

  return (
    <section>
      <form className="create" onSubmit={onInvite}>
        <label>
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </label>
        <label>
          Role
          <select value={roleId} onChange={(e) => setRoleId(e.target.value)} required>
            {roles.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
        </label>
        <button type="submit">Invite</button>
      </form>

      <ul className="list">
        {members.map((m) => (
          <li key={m.id} className="member">
            <strong>{m.user_name || m.user_email}</strong>
            <span>{m.user_email}</span>
            <span>{m.role_key}</span>
            <span className="status">{m.status}</span>
          </li>
        ))}
        {members.length === 0 && <li className="empty">No members yet.</li>}
      </ul>
    </section>
  )
}

function RoleEditor({
  orgId,
  roles,
  permissions,
  onChanged,
  onError,
}: {
  orgId: string
  roles: Role[]
  permissions: Permission[]
  onChanged: () => Promise<void>
  onError: (msg: string | null) => void
}) {
  const [selectedId, setSelectedId] = useState('')
  const [draftKeys, setDraftKeys] = useState<string[]>([])
  const [newKey, setNewKey] = useState('')
  const [newName, setNewName] = useState('')

  const selected = roles.find((r) => r.id === selectedId) ?? roles[0]
  const locked = selected?.key === 'owner'

  useEffect(() => {
    if (!selectedId && roles[0]) {
      setSelectedId(roles[0].id)
    }
  }, [roles, selectedId])

  useEffect(() => {
    if (selected) {
      setDraftKeys(selected.permission_keys ?? [])
    }
  }, [selected?.id, selected?.permission_keys])

  const byCategory = useMemo(() => {
    const map = new Map<string, Permission[]>()
    for (const p of permissions) {
      const list = map.get(p.category) ?? []
      list.push(p)
      map.set(p.category, list)
    }
    return [...map.entries()]
  }, [permissions])

  function toggle(key: string) {
    if (locked) return
    setDraftKeys((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key],
    )
  }

  async function onSave(e: FormEvent) {
    e.preventDefault()
    if (!selected || locked) return
    onError(null)
    try {
      await updateRole(selected.id, draftKeys)
      await onChanged()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Save role failed')
    }
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      const role = await createRole(orgId, {
        key: newKey,
        name: newName,
        permission_keys: draftKeys.length ? draftKeys : ['members:read'],
      })
      setNewKey('')
      setNewName('')
      await onChanged()
      setSelectedId(role.id)
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Create role failed')
    }
  }

  return (
    <section className="roles">
      <form className="create" onSubmit={onCreate}>
        <label>
          New role key
          <input value={newKey} onChange={(e) => setNewKey(e.target.value)} required />
        </label>
        <label>
          Name
          <input value={newName} onChange={(e) => setNewName(e.target.value)} required />
        </label>
        <button type="submit">Create role</button>
      </form>

      <div className="role-toolbar">
        <label>
          Edit role
          <select value={selected?.id ?? ''} onChange={(e) => setSelectedId(e.target.value)}>
            {roles.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name} ({r.key})
              </option>
            ))}
          </select>
        </label>
        {locked && <p className="status">Owner permissions are locked.</p>}
      </div>

      <form onSubmit={onSave}>
        {byCategory.map(([category, perms]) => (
          <fieldset key={category} className="perm-group">
            <legend>{category}</legend>
            {perms.map((p) => (
              <label key={p.key} className="perm">
                <input
                  type="checkbox"
                  checked={draftKeys.includes(p.key)}
                  disabled={locked}
                  onChange={() => toggle(p.key)}
                />
                <span>
                  <strong>{p.key}</strong>
                  <em>{p.description}</em>
                </span>
              </label>
            ))}
          </fieldset>
        ))}
        <button type="submit" disabled={locked || !selected}>
          Save permissions
        </button>
      </form>
    </section>
  )
}

function BillingPanel({
  orgId,
  onError,
}: {
  orgId: string
  onError: (msg: string | null) => void
}) {
  const [plans, setPlans] = useState<Plan[]>([])
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [effective, setEffective] = useState<string[]>([])

  async function refresh() {
    onError(null)
    try {
      const [p, e] = await Promise.all([listPlans(), getOrgEntitlements(orgId)])
      setPlans(p.items)
      setEffective(e.entitlements)
      try {
        const sub = await getSubscription(orgId)
        setSubscription(sub)
      } catch {
        setSubscription(null)
      }
      await listEntitlementsCatalog()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Billing load failed')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  const currentPlan = plans.find((p) => p.id === subscription?.plan_id)

  return (
    <section className="billing">
      <p className="status">
        Subscription: {subscription ? `${subscription.status}` : 'none'}
        {currentPlan ? ` · ${currentPlan.name} (${currentPlan.key})` : ''}
      </p>
      <p>Effective entitlements: {effective.length ? effective.join(', ') : 'none'}</p>
      <p className="lede">Plan changes are managed by platform admins until self-serve billing ships.</p>
    </section>
  )
}
