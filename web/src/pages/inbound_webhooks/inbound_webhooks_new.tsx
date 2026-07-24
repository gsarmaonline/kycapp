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
  const [secret, setSecret] = useState('')
  const [status, setStatus] = useState('connected')
  const [error, setError] = useState<string | null>(null)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [createdURL, setCreatedURL] = useState<string | null>(null)
  const [createdId, setCreatedId] = useState<string | null>(null)
  const [createdStatus, setCreatedStatus] = useState('connected')
  const [createdAuthMode, setCreatedAuthMode] = useState('header')

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const row = await createInboundWebhook(orgId, {
        name,
        auth_mode: authMode,
        status,
        ...(secret.trim() ? { secret: secret.trim() } : {}),
      })
      setCreatedSecret(row.secret ?? null)
      setCreatedURL(row.url)
      setCreatedId(row.id)
      setCreatedStatus(row.status)
      setCreatedAuthMode(row.auth_mode || authMode)
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
        {createdAuthMode === 'header' && createdSecret && (
          <p className="notice">
            Header secret (copy now): <code>{createdSecret}</code>
          </p>
        )}
        {createdStatus === 'disconnected' && (
          <p className="field-hint">
            Status is <strong>disconnected</strong> — connect it before the source can call this
            endpoint.
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
        <label>
          Secret (optional)
          <input
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            autoComplete="new-password"
            placeholder="Leave blank to auto-generate"
          />
        </label>
        <p className="field-hint">
          Leave blank to generate a secret. For query/path modes it becomes part of the URL.
        </p>
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="connected">connected (accept traffic)</option>
            <option value="disconnected">disconnected (reject until enabled)</option>
          </select>
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'inbound-webhooks')} submitLabel="Create" />
      </form>
    </section>
  )
}
