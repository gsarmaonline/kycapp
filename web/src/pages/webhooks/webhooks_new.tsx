import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createOrgWebhook } from '../../api'
import { VariableDocsHint } from '../../components/VariableDocsHint'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

const EXAMPLE_BODY = `{
  "organisation_id": "{{organisation_id}}",
  "trigger": "{{trigger}}",
  "id": "{{app_user.id}}",
  "email": "{{app_user.email}}",
  "country": "{{app_user.country}}"
}`

export function WebhooksNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [bodyTemplate, setBodyTemplate] = useState(EXAMPLE_BODY)
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const row = await createOrgWebhook(orgId, {
        name,
        url,
        secret,
        body_template: bodyTemplate,
      })
      navigate(resourcePath(orgId, 'webhooks', row.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Add outbound webhook" />
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
        <VariableDocsHint>
          Use <code>{'{{path}}'}</code> placeholders (e.g. <code>{'{{app_user.email}}'}</code>). Empty
          template sends the full event dump.
        </VariableDocsHint>
        <button
          type="button"
          className="ghost"
          onClick={() => setBodyTemplate(EXAMPLE_BODY)}
        >
          Reset to example
        </button>
        <FormActions cancelTo={resourcePath(orgId, 'webhooks')} submitLabel="Create" />
      </form>
    </section>
  )
}
