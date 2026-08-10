import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getAppScopeType, updateAppScopeType } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerScopeKindsEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [kind, setKind] = useState('')
  const [label, setLabel] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAppScopeType(orgId, id)
      .then((s) => {
        setKind(s.kind)
        setLabel(s.label)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateAppScopeType(orgId, id, { label })
      navigate(resourcePath(orgId, 'customer-scope-kinds'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <section>
      <PageHeader title="Edit scope kind" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Kind
          {/* Fixed: grants and your own resources refer to it. */}
          <input value={kind} disabled />
        </label>
        <label>
          Label
          <input value={label} onChange={(e) => setLabel(e.target.value)} />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'customer-scope-kinds')} submitLabel="Save" />
      </form>
    </section>
  )
}
