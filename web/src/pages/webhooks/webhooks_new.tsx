import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createOrgWebhook } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function WebhooksNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const row = await createOrgWebhook(orgId, { name, url, secret })
      navigate(resourcePath(orgId, 'webhooks', row.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Add webhook" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={(e) => void onSubmit(e)}>
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
        <FormActions cancelTo={resourcePath(orgId, 'webhooks')} submitLabel="Create" />
      </form>
    </section>
  )
}
