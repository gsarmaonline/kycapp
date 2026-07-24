import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getOrgWebhook, updateOrgWebhook } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

const EXAMPLE_BODY = `{
  "organisation_id": "{{organisation_id}}",
  "trigger": "{{trigger}}",
  "id": "{{app_user.id}}",
  "email": "{{app_user.email}}",
  "country": "{{app_user.country}}"
}`

export function WebhooksEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [status, setStatus] = useState('connected')
  const [bodyTemplate, setBodyTemplate] = useState('')
  const [hasSecret, setHasSecret] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void getOrgWebhook(orgId, id)
      .then((w) => {
        setName(w.name)
        setUrl(w.url)
        setStatus(w.status)
        setHasSecret(w.has_secret)
        setBodyTemplate(w.body_template || '')
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [orgId, id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateOrgWebhook(orgId, id, {
        name,
        url,
        secret: secret || undefined,
        status,
        body_template: bodyTemplate,
      })
      navigate(resourcePath(orgId, 'webhooks', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit webhook" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={(e) => void onSubmit(e)}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          URL
          <input value={url} onChange={(e) => setUrl(e.target.value)} required />
        </label>
        <label>
          Shared secret
          <input
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            autoComplete="new-password"
            placeholder={hasSecret ? 'Leave blank to keep current' : 'Optional'}
          />
        </label>
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="connected">connected</option>
            <option value="disconnected">disconnected</option>
          </select>
        </label>
        <label>
          Body template (JSON)
          <textarea
            value={bodyTemplate}
            onChange={(e) => setBodyTemplate(e.target.value)}
            rows={10}
            spellCheck={false}
            placeholder="Leave empty to POST { organisation_id, payload }"
          />
        </label>
        <p className="field-hint">
          Use <code>{'{{app_user.email}}'}</code>-style paths (same as automation conditions). Empty
          template sends the full event dump.
        </p>
        <button type="button" className="ghost" onClick={() => setBodyTemplate(EXAMPLE_BODY)}>
          Use example template
        </button>
        <FormActions cancelTo={resourcePath(orgId, 'webhooks', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
