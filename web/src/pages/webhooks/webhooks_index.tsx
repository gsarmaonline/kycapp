import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteOrgWebhook,
  getInboundHook,
  listOrgWebhooks,
  updateInboundHook,
  type InboundHook,
  type OrgWebhook,
} from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function WebhooksIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<OrgWebhook[]>([])
  const [inbound, setInbound] = useState<InboundHook | null>(null)
  const [revealedSecret, setRevealedSecret] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const [wh, hook] = await Promise.all([listOrgWebhooks(orgId), getInboundHook(orgId)])
      setItems(wh.items)
      setInbound(hook)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load webhooks')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(w: OrgWebhook) {
    if (!confirm(`Delete webhook ${w.name}? Automations that use it will fail until updated.`)) {
      return
    }
    try {
      await deleteOrgWebhook(orgId, w.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  async function onRotateInbound() {
    if (
      !confirm(
        'Generate a new inbound secret? External systems must be updated with the new value.',
      )
    ) {
      return
    }
    try {
      const next = await updateInboundHook(orgId, { rotate: true })
      setInbound(next)
      setRevealedSecret(next.secret ?? null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Rotate failed')
    }
  }

  async function onToggleInbound() {
    if (!inbound) return
    const nextStatus = inbound.status === 'connected' ? 'disconnected' : 'connected'
    if (nextStatus === 'connected' && !inbound.has_secret) {
      await onRotateInbound()
      return
    }
    try {
      const next = await updateInboundHook(orgId, { status: nextStatus })
      setInbound(next)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Webhooks"
        createTo={resourcePath(orgId, 'webhooks', 'new')}
        createLabel="Add outbound"
      />
      <p className="lede">
        Outbound endpoints for <code>call_webhook</code>, plus one inbound URL that fires{' '}
        <code>webhook.received</code>.
      </p>
      {error && <p className="error">{error}</p>}

      <h3>Inbound (automation trigger)</h3>
      {loading || !inbound ? (
        <p>Loading…</p>
      ) : (
        <div className="inbound-hook-card">
          <p className="field-hint">
            <code>POST</code> JSON with header <code>X-KYC-Webhook-Secret</code>. Subject is the
            organisation — use with webhook / db_insert / delay actions (not send_email).
          </p>
          <label>
            Endpoint URL
            <input value={inbound.url} readOnly onFocus={(e) => e.target.select()} />
          </label>
          <p>
            Status: <strong>{inbound.status}</strong>
            {inbound.has_secret ? ` · secret ${inbound.secret_hint || '••••'}` : ' · no secret'}
          </p>
          {revealedSecret && (
            <p className="notice">
              New secret (copy now): <code>{revealedSecret}</code>
            </p>
          )}
          <div className="form-actions" style={{ justifyContent: 'flex-start' }}>
            <button type="button" className="ghost" onClick={() => void onRotateInbound()}>
              {inbound.has_secret ? 'Rotate secret' : 'Enable & generate secret'}
            </button>
            {inbound.has_secret && (
              <button type="button" className="ghost" onClick={() => void onToggleInbound()}>
                {inbound.status === 'connected' ? 'Disconnect' : 'Connect'}
              </button>
            )}
          </div>
        </div>
      )}

      <h3>Outbound (call_webhook)</h3>
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'URL', 'Secret', 'Status']}
          empty="No outbound webhooks yet."
          rows={items.map((w) => ({
            key: w.id,
            cells: [
              w.name,
              w.url,
              w.has_secret ? w.secret_hint || '••••' : '—',
              w.status,
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'webhooks', w.id)}
                editTo={resourcePath(orgId, 'webhooks', w.id, 'edit')}
                onDelete={() => void onDelete(w)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
