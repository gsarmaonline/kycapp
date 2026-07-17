import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
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
type View = { name: 'list' } | { name: 'detail'; orgId: string }

export default function App() {
  const [gate, setGate] = useState<Gate>('loading')
  const [user, setUser] = useState<User | null>(null)
  const [view, setView] = useState<View>({ name: 'list' })

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
  }

  async function onLogout() {
    await logout()
    setUser(null)
    setView({ name: 'list' })
    setGate('auth')
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

  if (view.name === 'detail') {
    return (
      <OrganisationDetail
        orgId={view.orgId}
        user={user}
        onBack={() => setView({ name: 'list' })}
        onLogout={onLogout}
      />
    )
  }

  return (
    <OrganisationList
      user={user}
      onOpen={(id) => setView({ name: 'detail', orgId: id })}
      onLogout={onLogout}
    />
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

function OrganisationList({
  user,
  onOpen,
  onLogout,
}: {
  user: User | null
  onOpen: (id: string) => void
  onLogout: () => Promise<void>
}) {
  const [items, setItems] = useState<Organisation[]>([])
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const res = await listOrganisations()
      setItems(res.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const org = await createOrganisation(name)
      setName('')
      await refresh()
      onOpen(org.id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <main className="page">
      <header className="topbar">
        <div>
          <p className="eyebrow">KYC</p>
          <h1>Your organisations</h1>
          {user && <p className="lede">{user.email}</p>}
        </div>
        <button type="button" className="ghost" onClick={() => void onLogout()}>
          Sign out
        </button>
      </header>

      <form className="create" onSubmit={onCreate}>
        <label>
          Name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Acme Pty Ltd"
            required
          />
        </label>
        <button type="submit">Create</button>
      </form>

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ul className="list">
          {items.map((org) => (
            <li key={org.id}>
              <button type="button" className="linkish" onClick={() => onOpen(org.id)}>
                <strong>{org.name}</strong>
                <span>{org.slug}</span>
                <span className="status">{org.status}</span>
              </button>
            </li>
          ))}
          {items.length === 0 && <li className="empty">No organisations yet — create one above.</li>}
        </ul>
      )}
    </main>
  )
}

function OrganisationDetail({
  orgId,
  user,
  onBack,
  onLogout,
}: {
  orgId: string
  user: User | null
  onBack: () => void
  onLogout: () => Promise<void>
}) {
  const [org, setOrg] = useState<Organisation | null>(null)
  const [members, setMembers] = useState<Membership[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [email, setEmail] = useState('')
  const [roleId, setRoleId] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function refresh() {
    setError(null)
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
      const memberRole = r.items.find((x) => x.key === 'member')
      setRoleId((prev) => prev || memberRole?.id || r.items[0]?.id || '')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onInvite(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await inviteMember(orgId, email, roleId)
      setEmail('')
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invite failed')
    }
  }

  return (
    <main className="page">
      <header className="topbar">
        <button type="button" className="back" onClick={onBack}>
          ← Organisations
        </button>
        <button type="button" className="ghost" onClick={() => void onLogout()}>
          Sign out
        </button>
      </header>
      {org && (
        <header>
          <p className="eyebrow">{org.slug}</p>
          <h1>{org.name}</h1>
          <p className="status">{org.status}</p>
          {user && <p className="lede">Signed in as {user.email}</p>}
        </header>
      )}

      <section>
        <h2>Members</h2>
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

        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}

        <ul className="list">
          {members.map((m) => (
            <li key={m.id} className="member">
              <strong>{m.user_name || m.user_email}</strong>
              <span>{m.user_email}</span>
              <span>{m.role_key}</span>
              <span className="status">{m.status}</span>
            </li>
          ))}
        </ul>
      </section>

      <RoleEditor
        orgId={orgId}
        roles={roles}
        permissions={permissions}
        onChanged={refresh}
        onError={setError}
      />

      <BillingPanel orgId={orgId} onError={setError} />
    </main>
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
      <h2>Roles</h2>
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

      <form className="create role-create" onSubmit={onCreate}>
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
      <h2>Billing</h2>
      <p className="status">
        Subscription: {subscription ? `${subscription.status}` : 'none'}
        {currentPlan ? ` · ${currentPlan.name} (${currentPlan.key})` : ''}
      </p>
      <p>Effective entitlements: {effective.length ? effective.join(', ') : 'none'}</p>
      <p className="lede">Plan changes are managed by platform admins until self-serve billing ships.</p>
    </section>
  )
}
