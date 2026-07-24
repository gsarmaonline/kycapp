import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getOrgDatabase, updateOrgDatabase } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function DatabasesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('5432')
  const [database, setDatabase] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [sslMode, setSslMode] = useState('require')
  const [status, setStatus] = useState('')
  const [lastError, setLastError] = useState('')
  const [hasPassword, setHasPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void getOrgDatabase(orgId, id)
      .then((d) => {
        setName(d.name)
        setHost(d.host)
        setPort(String(d.port))
        setDatabase(d.database_name)
        setUsername(d.username)
        setSslMode(d.ssl_mode || 'require')
        setStatus(d.status)
        setLastError(d.last_error || '')
        setHasPassword(d.has_password)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [orgId, id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const row = await updateOrgDatabase(orgId, id, {
        name,
        host,
        port: Number(port) || 5432,
        database_name: database,
        username,
        password: password || undefined,
        ssl_mode: sslMode,
      })
      if (row.status !== 'connected') {
        setStatus(row.status)
        setLastError(row.last_error || '')
        setError(
          `Saved, but database is unreachable: ${row.last_error || 'connection failed'}`,
        )
        return
      }
      navigate(resourcePath(orgId, 'databases', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit database" />
      {error && <p className="error">{error}</p>}
      {status && (
        <p className="status">
          Status: <strong>{status}</strong>
          {lastError ? ` — ${lastError}` : ''}
        </p>
      )}
      <p className="field-hint">Saving re-tests the connection and updates status.</p>
      <form className="create stacked" onSubmit={(e) => void onSubmit(e)}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Host
          <input value={host} onChange={(e) => setHost(e.target.value)} required />
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
            autoComplete="new-password"
            placeholder={hasPassword ? 'Leave blank to keep current' : 'Required'}
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
        <FormActions cancelTo={resourcePath(orgId, 'databases', id)} submitLabel="Save & test" />
      </form>
    </section>
  )
}
