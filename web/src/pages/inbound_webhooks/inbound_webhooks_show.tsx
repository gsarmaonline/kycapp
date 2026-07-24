import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getInboundWebhook, updateInboundWebhook, type InboundWebhook } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

const AUTH_LABELS: Record<string, string> = {
  header: 'Header X-KYC-Webhook-Secret',
  query: 'Query ?secret=',
  path: 'URL path token',
}

export function InboundWebhooksShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<InboundWebhook | null>(null)
  const [revealed, setRevealed] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getInboundWebhook(orgId, id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  async function onRotate() {
    if (!confirm('Rotate secret? The endpoint URL may change for query/path modes.')) return
    try {
      const next = await updateInboundWebhook(orgId, id, { rotate: true })
      setItem(next)
      setRevealed(next.secret ?? null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Rotate failed')
    }
  }

  async function onToggle() {
    if (!item) return
    const status = item.status === 'connected' ? 'disconnected' : 'connected'
    try {
      setItem(await updateInboundWebhook(orgId, id, { status }))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed')
    }
  }

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  const mode = item.auth_mode || 'header'

  return (
    <section>
      <PageHeader title={item.name || 'Inbound webhook'} />
      <DetailList
        items={[
          { label: 'Name', value: item.name },
          {
            label: 'Endpoint URL (give to source)',
            value: (
              <input
                value={item.url}
                readOnly
                onFocus={(e) => e.target.select()}
                style={{ width: '100%', font: 'inherit' }}
              />
            ),
          },
          { label: 'Auth mode', value: AUTH_LABELS[mode] || mode },
          {
            label: 'Secret',
            value: item.has_secret ? item.secret_hint || '••••' : '—',
          },
          { label: 'Status', value: item.status },
        ]}
      />
      {revealed && mode === 'header' && (
        <p className="notice">
          New header secret (copy now): <code>{revealed}</code>
        </p>
      )}
      {mode !== 'header' && (
        <p className="field-hint">
          Query/path modes embed the secret in the URL above — paste that full URL into the source
          system.
        </p>
      )}
      <p className="muted">
        Fires <code>webhook.received</code> with <code>inbound_webhook_id</code> /{' '}
        <code>inbound_webhook_name</code> and <code>body</code> (JSON if possible, otherwise raw
        text).
      </p>
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'inbound-webhooks')}>
          Back
        </Link>
        <button type="button" className="ghost" onClick={() => void onRotate()}>
          Rotate secret
        </button>
        <button type="button" className="ghost" onClick={() => void onToggle()}>
          {item.status === 'connected' ? 'Disconnect' : 'Connect'}
        </button>
        <Link className="button" to={resourcePath(orgId, 'inbound-webhooks', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
