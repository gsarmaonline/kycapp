import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import {
  createOrganisation,
  getOrganisation,
  inviteMember,
  listMemberships,
  listOrganisations,
  listRoles,
  type Membership,
  type Organisation,
  type Role,
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
  const [email, setEmail] = useState('')
  const [roleId, setRoleId] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function refresh() {
    setError(null)
    try {
      const [o, m, r] = await Promise.all([
        getOrganisation(orgId),
        listMemberships(orgId),
        listRoles(orgId),
      ])
      setOrg(o)
      setMembers(m.items)
      setRoles(r.items)
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
    </main>
  )
}
