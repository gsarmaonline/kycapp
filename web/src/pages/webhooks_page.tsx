import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import {
  createOrgWebhook,
  deleteOrgWebhook,
  listOrgWebhooks,
  type OrgWebhook,
} from '../api'
import { PageHeader } from '../crud/ui'

export function WebhooksPage() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<OrgWebhook[]>([])
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const res = await listOrgWebhooks(orgId)
      setItems(res.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load webhooks')
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
      const row = await createOrgWebhook(orgId, { name, url, secret })
      setItems((prev) => [...prev, row].sort((a, b) => a.name.localeCompare(b.name)))
      setName('')
      setUrl('')
      setSecret('')
      setMessage(`Webhook “${row.name}” saved`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete(id: string, label: string) {
    if (!confirm(`Remove webhook “${label}”? Automations that use it will fail until updated.`)) {
      return
    }
    setBusy(true)
    setError(null)
    try {
      await deleteOrgWebhook(orgId, id)
      setItems((prev) => prev.filter((w) => w.id !== id))
      setMessage('Webhook removed')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setBusy(false)
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Webhooks" />
      <p className="lede">
        Outbound endpoints for the <code>call_webhook</code> automation action. KYC POSTs{' '}
        <code>{'{ organisation_id, payload }'}</code> as JSON. Optional shared secret is sent as{' '}
        <code>X-KYC-Webhook-Secret</code>.
      </p>
      {error && <p className="error">{error}</p>}
      {message && <p className="status">{message}</p>}

      {items.length === 0 ? (
        <p className="muted">No webhooks configured yet.</p>
      ) : (
        <ul className="run-list">
          {items.map((w) => (
            <li key={w.id}>
              <strong>{w.name}</strong> · <code>{w.url}</code> ·{' '}
              {w.has_secret ? `secret ${w.secret_hint}` : 'no secret'} · {w.status}{' '}
              <button
                type="button"
                className="ghost"
                disabled={busy}
                onClick={() => void onDelete(w.id, w.name)}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}

      <form className="create stacked" onSubmit={(e) => void onCreate(e)}>
        <h3>Add webhook</h3>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required placeholder="CRM sync" />
        </label>
        <label>
          URL
          <input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
            placeholder="https://api.example.com/hooks/kyc"
          />
        </label>
        <label>
          Shared secret (optional)
          <input
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            autoComplete="new-password"
            placeholder="Optional"
          />
        </label>
        <button type="submit" disabled={busy}>
          Add webhook
        </button>
      </form>
    </section>
  )
}
