import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getOrgWebhook, type OrgWebhook } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function WebhooksShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<OrgWebhook | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getOrgWebhook(orgId, id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title={item.name || 'Webhook'} />
      <DetailList
        items={[
          { label: 'Name', value: item.name },
          { label: 'URL', value: <code>{item.url}</code> },
          {
            label: 'Secret',
            value: item.has_secret ? item.secret_hint || '••••' : '—',
          },
          { label: 'Status', value: item.status },
        ]}
      />
      <p className="muted">
        Automations POST <code>{'{ organisation_id, payload }'}</code> as JSON. When set, the shared
        secret is sent as <code>X-KYC-Webhook-Secret</code>.
      </p>
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'webhooks')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'webhooks', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
