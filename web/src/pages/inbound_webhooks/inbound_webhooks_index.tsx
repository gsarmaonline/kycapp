import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteInboundWebhook,
  listInboundWebhooks,
  type InboundWebhook,
} from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function InboundWebhooksIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<InboundWebhook[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listInboundWebhooks(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load inbound webhooks')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(w: InboundWebhook) {
    if (!confirm(`Delete inbound webhook ${w.name}?`)) return
    try {
      await deleteInboundWebhook(orgId, w.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Inbound webhooks"
        createTo={resourcePath(orgId, 'inbound-webhooks', 'new')}
        createLabel="Add inbound"
      />
      <p className="lede">
        Public endpoints that fire the <code>webhook.received</code> automation trigger. Subject is
        the organisation. For sending from automations, use{' '}
        <a href={resourcePath(orgId, 'webhooks')}>Outbound webhooks</a>.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Auth', 'Status']}
          empty="No inbound webhooks yet."
          rows={items.map((w) => ({
            key: w.id,
            cells: [w.name, w.auth_mode || 'header', w.status],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'inbound-webhooks', w.id)}
                editTo={resourcePath(orgId, 'inbound-webhooks', w.id, 'edit')}
                onDelete={() => void onDelete(w)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
