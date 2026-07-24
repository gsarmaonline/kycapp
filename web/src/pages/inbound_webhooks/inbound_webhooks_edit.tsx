import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getInboundWebhook, updateInboundWebhook } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function InboundWebhooksEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [status, setStatus] = useState('connected')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getInboundWebhook(orgId, id)
      .then((w) => {
        setName(w.name)
        setStatus(w.status)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateInboundWebhook(orgId, id, { name, status })
      navigate(resourcePath(orgId, 'inbound-webhooks', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <section>
      <PageHeader title="Edit inbound webhook" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={(e) => void onSubmit(e)}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="connected">connected</option>
            <option value="disconnected">disconnected</option>
          </select>
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'inbound-webhooks', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
