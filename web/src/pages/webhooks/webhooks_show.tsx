import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
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

  const body = item.body_template?.trim()
    ? item.body_template
    : '{ organisation_id, payload }  (full event dump)'

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
          {
            label: 'Body template',
            value: <pre className="code-block">{body}</pre>,
          },
        ]}
        editTo={resourcePath(orgId, 'webhooks', item.id, 'edit')}
        backTo={resourcePath(orgId, 'webhooks')}
      />
      <p className="muted">
        When set, the shared secret is sent as <code>X-KYC-Webhook-Secret</code>. Placeholders use{' '}
        <code>{'{{path}}'}</code> from the trigger payload.
      </p>
    </section>
  )
}
