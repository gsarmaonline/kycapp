import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteOrgWebhook, listOrgWebhooks, type OrgWebhook } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function WebhooksIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<OrgWebhook[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listOrgWebhooks(orgId)).items)
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

  return (
    <section>
      <PageHeader
        title="Webhooks"
        createTo={resourcePath(orgId, 'webhooks', 'new')}
        createLabel="Add webhook"
      />
      <p className="lede">
        Outbound endpoints for the <code>call_webhook</code> automation action.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'URL', 'Secret', 'Status']}
          empty="No webhooks yet."
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
