import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import {
  createOrgDatabase,
  deleteOrgDatabase,
  listOrgDatabases,
  type OrgDatabase,
} from '../api'
import { PageHeader } from '../crud/ui'

export function DatabasesPage() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<OrgDatabase[]>([])
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('5432')
  const [database, setDatabase] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [sslMode, setSslMode] = useState('require')
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const res = await listOrgDatabases(orgId)
      setItems(res.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load databases')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setMessage(null)
    try {
      const row = await createOrgDatabase(orgId, {
        name,
        host,
        port: Number(port) || 5432,
        database_name: database,
        username,
        password,
        ssl_mode: sslMode,
      })
      setItems((prev) => [...prev, row].sort((a, b) => a.name.localeCompare(b.name)))
      setName('')
      setHost('')
      setPort('5432')
      setDatabase('')
      setUsername('')
      setPassword('')
      setSslMode('require')
      setMessage(`Database “${row.name}” connected`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete(id: string, label: string) {
    if (!confirm(`Remove database “${label}”? Automations that use it will fail until updated.`)) {
      return
    }
    setBusy(true)
    setError(null)
    try {
      await deleteOrgDatabase(orgId, id)
      setItems((prev) => prev.filter((d) => d.id !== id))
      setMessage('Database removed')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setBusy(false)
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Databases" />
      <p className="lede">
        Postgres connections for the <code>db_insert</code> automation action. Prefer a
        least-privilege write user and a dedicated landing table. Passwords are never shown in
        full after save.
      </p>
      {error && <p className="error">{error}</p>}
      {message && <p className="status">{message}</p>}

      {items.length === 0 ? (
        <p className="muted">No databases connected yet.</p>
      ) : (
        <ul className="run-list">
          {items.map((d) => (
            <li key={d.id}>
              <strong>{d.name}</strong> · {d.username}@{d.host}:{d.port}/{d.database_name} ·{' '}
              {d.has_password ? d.password_hint : 'no password'} · {d.status}{' '}
              <button
                type="button"
                className="ghost"
                disabled={busy}
                onClick={() => void onDelete(d.id, d.name)}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}

      <form className="create stacked" onSubmit={(e) => void onCreate(e)}>
        <h3>Add database</h3>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Analytics dump" />
        </label>
        <label>
          Host
          <input value={host} onChange={(e) => setHost(e.target.value)} required placeholder="db.example.com" />
        </label>
        <label>
          Port
          <input value={port} onChange={(e) => setPort(e.target.value)} inputMode="numeric" />
        </label>
        <label>
          Database
          <input value={database} onChange={(e) => setDatabase(e.target.value)} required />
        </label>
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} required autoComplete="off" />
        </label>
        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="new-password"
          />
        </label>
        <label>
          SSL mode
          <select value={sslMode} onChange={(e) => setSslMode(e.target.value)}>
            <option value="require">require</option>
            <option value="verify-full">verify-full</option>
            <option value="prefer">prefer</option>
            <option value="disable">disable</option>
          </select>
        </label>
        <button type="submit" disabled={busy}>
          Add database
        </button>
      </form>
    </section>
  )
}
