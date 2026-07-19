import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getProductFeature, updateProductFeature } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function ProductFeaturesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void getProductFeature(id)
      .then((f) => {
        setKey(f.key)
        setDescription(f.description)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateProductFeature(id, { description })
      navigate(resourcePath(orgId, 'product-features', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title={`Edit ${key}`} />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input value={key} disabled />
        </label>
        <label>
          Description
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'product-features', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
