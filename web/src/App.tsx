import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import {
  createOrganisation,
  createPlan,
  createRole,
  getOrganisation,
  getOrgEntitlements,
  getSubscription,
  inviteMember,
  listEntitlementsCatalog,
  listMemberships,
  listOrganisations,
  listPermissions,
  listPlans,
  listRoles,
  setOrgEntitlements,
  setPlanEntitlements,
  updateRole,
  upsertSubscription,
  type Entitlement,
  type Membership,
  type Organisation,
  type Permission,
  type Plan,
  type Role,
  type Subscription,
} from './api'
import './App.css'

type View =
  | { name: 'list' }
  | { name: 'detail'; orgId: string }

export default function App() {
  const [view, setView] = useState<View>({ name: 'list' })

  if (view.name === 'detail') {
    return (
      <OrganisationDetail
        orgId={view.orgId}
        onBack={() => setView({ name: 'list' })}
      />
    )
  }

  return <OrganisationList onOpen={(id) => setView({ name: 'detail', orgId: id })} />
}

function OrganisationList({ onOpen }: { onOpen: (id: string) => void }) {
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
      <header>
        <p className="eyebrow">KYC ops</p>
        <h1>Organisations</h1>
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

      {error && <p className="error" role="alert">{error}</p>}
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
          {items.length === 0 && <li className="empty">No organisations yet.</li>}
        </ul>
      )}
    </main>
  )
}

function OrganisationDetail({
  orgId,
  onBack,
}: {
  orgId: string
  onBack: () => void
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
      <button type="button" className="back" onClick={onBack}>
        ← Organisations
      </button>
      {org && (
        <header>
          <p className="eyebrow">{org.slug}</p>
          <h1>{org.name}</h1>
          <p className="status">{org.status}</p>
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

        {error && <p className="error" role="alert">{error}</p>}

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
          <select
            value={selected?.id ?? ''}
            onChange={(e) => setSelectedId(e.target.value)}
          >
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
  const [catalog, setCatalog] = useState<Entitlement[]>([])
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [effective, setEffective] = useState<string[]>([])
  const [planId, setPlanId] = useState('')
  const [grantKey, setGrantKey] = useState('')
  const [denyKey, setDenyKey] = useState('')
  const [newPlanKey, setNewPlanKey] = useState('')
  const [newPlanName, setNewPlanName] = useState('')

  async function refresh() {
    onError(null)
    try {
      const [p, c, e] = await Promise.all([
        listPlans(),
        listEntitlementsCatalog(),
        getOrgEntitlements(orgId),
      ])
      setPlans(p.items)
      setCatalog(c.items)
      setEffective(e.entitlements)
      try {
        const sub = await getSubscription(orgId)
        setSubscription(sub)
        setPlanId(sub.plan_id)
      } catch {
        setSubscription(null)
        setPlanId(p.items[0]?.id ?? '')
      }
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Billing load failed')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onAssign(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      await upsertSubscription(orgId, planId, 'active')
      await refresh()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Assign plan failed')
    }
  }

  async function onOverride(e: FormEvent) {
    e.preventDefault()
    onError(null)
    const overrides: { key: string; effect: 'grant' | 'deny' }[] = []
    if (grantKey) overrides.push({ key: grantKey, effect: 'grant' })
    if (denyKey) overrides.push({ key: denyKey, effect: 'deny' })
    try {
      await setOrgEntitlements(orgId, overrides)
      await refresh()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Override failed')
    }
  }

  async function onCreatePlan(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      const plan = await createPlan(newPlanKey, newPlanName)
      const keys = catalog.map((c) => c.key)
      if (keys.length) {
        await setPlanEntitlements(plan.id, keys)
      }
      setNewPlanKey('')
      setNewPlanName('')
      await refresh()
      setPlanId(plan.id)
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Create plan failed')
    }
  }

  const currentPlan = plans.find((p) => p.id === subscription?.plan_id)

  return (
    <section className="billing">
      <h2>Billing</h2>
      <p className="status">
        Subscription: {subscription ? `${subscription.status}` : 'none'}
        {currentPlan ? ` · ${currentPlan.name} (${currentPlan.key})` : ''}
      </p>
      <p>
        Effective entitlements:{' '}
        {effective.length ? effective.join(', ') : 'none'}
      </p>

      <form className="create" onSubmit={onAssign}>
        <label>
          Plan
          <select value={planId} onChange={(e) => setPlanId(e.target.value)} required>
            {plans.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.key}) — {p.entitlement_keys.join(', ') || 'no entitlements'}
              </option>
            ))}
          </select>
        </label>
        <button type="submit">Assign plan</button>
      </form>

      <form className="create" onSubmit={onOverride}>
        <label>
          Grant
          <select value={grantKey} onChange={(e) => setGrantKey(e.target.value)}>
            <option value="">—</option>
            {catalog.map((c) => (
              <option key={c.key} value={c.key}>
                {c.key}
              </option>
            ))}
          </select>
        </label>
        <label>
          Deny
          <select value={denyKey} onChange={(e) => setDenyKey(e.target.value)}>
            <option value="">—</option>
            {catalog.map((c) => (
              <option key={c.key} value={c.key}>
                {c.key}
              </option>
            ))}
          </select>
        </label>
        <button type="submit">Set overrides</button>
      </form>

      <form className="create role-create" onSubmit={onCreatePlan}>
        <label>
          New plan key
          <input value={newPlanKey} onChange={(e) => setNewPlanKey(e.target.value)} required />
        </label>
        <label>
          Name
          <input value={newPlanName} onChange={(e) => setNewPlanName(e.target.value)} required />
        </label>
        <button type="submit">Create plan (all entitlements)</button>
      </form>

      <ul className="list">
        {plans.map((p) => (
          <li key={p.id} className="member">
            <strong>{p.name}</strong>
            <span>{p.key}</span>
            <span>{p.entitlement_keys.join(', ') || '—'}</span>
            <span className="status">{p.status}</span>
          </li>
        ))}
      </ul>
    </section>
  )
}
