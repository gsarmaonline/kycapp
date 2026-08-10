import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createAppScopeType } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerScopeKindsNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [kind, setKind] = useState('')
  const [label, setLabel] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await createAppScopeType(orgId, { kind, label })
      navigate(resourcePath(orgId, 'customer-scope-kinds'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Add scope kind" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Kind
          <input value={kind} onChange={(e) => setKind(e.target.value)} placeholder="project" required />
        </label>
        <label>
          Label
          <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Project" />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'customer-scope-kinds')} submitLabel="Add scope kind" />
      </form>
    </section>
  )
}
