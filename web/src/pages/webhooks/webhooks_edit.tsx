import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getOrgWebhook, updateOrgWebhook } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function WebhooksEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [status, setStatus] = useState('connected')
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
        <FormActions cancelTo={resourcePath(orgId, 'webhooks', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
