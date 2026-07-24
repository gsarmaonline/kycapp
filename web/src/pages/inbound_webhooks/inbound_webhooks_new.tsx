import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createInboundWebhook } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function InboundWebhooksNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [createdId, setCreatedId] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const row = await createInboundWebhook(orgId, { name })
      setCreatedSecret(row.secret ?? null)
      setCreatedId(row.id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  if (createdId) {
    return (
      <section>
        <PageHeader title="Inbound webhook created" />
        {createdSecret && (
          <p className="notice">
            Secret (copy now — shown once): <code>{createdSecret}</code>
          </p>
        )}
        <p className="field-hint">
          Send <code>POST</code> with header <code>X-KYC-Webhook-Secret</code> to the endpoint URL on
          the detail page.
        </p>
        <div className="form-actions">
          <button type="button" onClick={() => navigate(resourcePath(orgId, 'inbound-webhooks', createdId))}>
            View endpoint
          </button>
        </div>
      </section>
    )
  }

  return (
    <section>
      <PageHeader title="Add inbound webhook" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={(e) => void onSubmit(e)}>
        <label>
          Name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            placeholder="Partner events"
          />
        </label>
        <p className="field-hint">A secret is generated automatically. Status starts as connected.</p>
        <FormActions cancelTo={resourcePath(orgId, 'inbound-webhooks')} submitLabel="Create" />
      </form>
    </section>
  )
}
