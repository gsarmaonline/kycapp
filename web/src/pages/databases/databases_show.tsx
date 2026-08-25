import { useEffect, useState } from 'react'
import { useLocation, useParams } from 'react-router-dom'
import {
  checkOrgDatabase,
  disconnectOrgDatabase,
  getOrgDatabase,
  type OrgDatabase,
} from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function DatabasesShow() {
  const { orgId = '', id = '' } = useParams()
  const location = useLocation()
  const [item, setItem] = useState<OrgDatabase | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(
    typeof (location.state as { warning?: string } | null)?.warning === 'string'
      ? (location.state as { warning: string }).warning
      : null,
  )
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void getOrgDatabase(orgId, id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  async function onCheck() {
    setBusy(true)
    setError(null)
    setMessage(null)
    try {
      const row = await checkOrgDatabase(orgId, id)
      setItem(row)
      setMessage(
        row.status === 'connected'
          ? 'Connection OK'
          : `Unreachable: ${row.last_error || 'unknown error'}`,
      )
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Check failed')
    } finally {
      setBusy(false)
    }
  }

  async function onDisconnect() {
    if (!confirm('Mark this database as disconnected? Automations will stop using it until re-checked.')) {
      return
    }
    setBusy(true)
    setError(null)
    setMessage(null)
    try {
      const row = await disconnectOrgDatabase(orgId, id)
      setItem(row)
      setMessage('Marked disconnected')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Disconnect failed')
    } finally {
      setBusy(false)
    }
  }

  if (error && !item) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title={item.name || 'Database'} />
      {error && <p className="error">{error}</p>}
      {message && <p className="status">{message}</p>}
      <DetailList
        items={[
          { label: 'Name', value: item.name },
          { label: 'Driver', value: item.driver },
          { label: 'Host', value: item.host },
          { label: 'Port', value: String(item.port) },
          { label: 'Database', value: item.database_name },
          { label: 'Username', value: item.username },
          { label: 'Password', value: item.has_password ? item.password_hint || '••••' : '—' },
          { label: 'SSL mode', value: item.ssl_mode },
          { label: 'Status', value: item.status },
          {
            label: 'Last checked',
            value: item.last_checked_at
              ? new Date(item.last_checked_at).toLocaleString()
              : '—',
          },
          { label: 'Last error', value: item.last_error || '—' },
        ]}
        editTo={resourcePath(orgId, 'databases', item.id, 'edit')}
        backTo={resourcePath(orgId, 'databases')}
        actions={
          <>
            <button type="button" className="ghost" disabled={busy} onClick={() => void onCheck()}>
              Test connection
            </button>
            {item.status !== 'disconnected' && (
              <button
                type="button"
                className="ghost"
                disabled={busy}
                onClick={() => void onDisconnect()}
              >
                Disconnect
              </button>
            )}
          </>
        }
      />
    </section>
  )
}
