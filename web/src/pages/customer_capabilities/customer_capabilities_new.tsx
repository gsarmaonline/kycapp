import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createAppCapability } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerCapabilitiesNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await createAppCapability(orgId, { key, description })
      navigate(resourcePath(orgId, 'customer-capabilities'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Add capability" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="deploy:production"
            required
          />
        </label>
        <label>
          Description
          <input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="What it allows"
          />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'customer-capabilities')} submitLabel="Add capability" />
      </form>
    </section>
  )
}
