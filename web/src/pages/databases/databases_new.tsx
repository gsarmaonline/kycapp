import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createOrgDatabase } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function DatabasesNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('5432')
  const [database, setDatabase] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [sslMode, setSslMode] = useState('require')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
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
      navigate(resourcePath(orgId, 'databases', row.id), {
        state:
          row.status === 'connected'
            ? undefined
            : { warning: row.last_error || 'Saved but unreachable' },
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Add database" />
      {error && <p className="error">{error}</p>}
      <p className="field-hint">
        After save we probe <code>SELECT 1</code> and set status to <code>connected</code> or{' '}
        <code>unreachable</code>.
      </p>
      <form className="create stacked" onSubmit={(e) => void onSubmit(e)}>
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
        <FormActions cancelTo={resourcePath(orgId, 'databases')} submitLabel="Create & test" />
      </form>
    </section>
  )
}
