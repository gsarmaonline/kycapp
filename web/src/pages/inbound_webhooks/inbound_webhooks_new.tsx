import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createInboundWebhook } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

const AUTH_HINTS: Record<string, string> = {
  header: 'Caller must send X-KYC-Webhook-Secret. Use when the source supports custom headers.',
  query: 'Secret is in the URL as ?secret=. Use when the source only lets you set a URL.',
  path: 'Secret is the last path segment. Use when the source only lets you set a URL (no query).',
}

export function InboundWebhooksNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [authMode, setAuthMode] = useState('header')
  const [error, setError] = useState<string | null>(null)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [createdURL, setCreatedURL] = useState<string | null>(null)
  const [createdId, setCreatedId] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const row = await createInboundWebhook(orgId, { name, auth_mode: authMode })
      setCreatedSecret(row.secret ?? null)
      setCreatedURL(row.url)
      setCreatedId(row.id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  if (createdId) {
    return (
      <section>
        <PageHeader title="Inbound webhook created" />
        <p className="notice">
          Endpoint URL (give this to the source):
          <br />
          <code>{createdURL}</code>
        </p>
        {authMode === 'header' && createdSecret && (
          <p className="notice">
            Header secret (copy now): <code>{createdSecret}</code>
          </p>
        )}
        <div className="form-actions">
          <button
            type="button"
            onClick={() => navigate(resourcePath(orgId, 'inbound-webhooks', createdId))}
          >
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
        <label>
          How the source authenticates
          <select value={authMode} onChange={(e) => setAuthMode(e.target.value)}>
            <option value="header">Custom header (X-KYC-Webhook-Secret)</option>
            <option value="query">Secret in query string (?secret=)</option>
            <option value="path">Secret in URL path</option>
          </select>
        </label>
        <p className="field-hint">{AUTH_HINTS[authMode]}</p>
        <FormActions cancelTo={resourcePath(orgId, 'inbound-webhooks')} submitLabel="Create" />
      </form>
    </section>
  )
}
