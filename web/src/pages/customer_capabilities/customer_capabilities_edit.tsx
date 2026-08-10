import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getAppCapability, updateAppCapability } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerCapabilitiesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAppCapability(orgId, id)
      .then((c) => {
        setKey(c.key)
        setDescription(c.description)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateAppCapability(orgId, id, { description })
      navigate(resourcePath(orgId, 'customer-capabilities'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <section>
      <PageHeader title="Edit capability" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          {/* Fixed: roles refer to it, and changing it would silently drop the
              capability from every role holding it. */}
          <input value={key} disabled />
        </label>
        <label>
          Description
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'customer-capabilities')} submitLabel="Save" />
      </form>
    </section>
  )
}
