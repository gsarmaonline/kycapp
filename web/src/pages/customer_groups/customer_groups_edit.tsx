import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getAppUserGroup, updateAppUserGroup } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { GroupParentsField } from './group_parents_field'
import { resourcePath } from '../../org_nav'

export function CustomerGroupsEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [parents, setParents] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAppUserGroup(orgId, id)
      .then((g) => {
        setKey(g.key)
        setName(g.name)
        setDescription(g.description)
        setParents(g.parents ?? [])
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateAppUserGroup(orgId, id, { name, description, parents })
      navigate(resourcePath(orgId, 'customer-groups'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <section>
      <PageHeader title="Edit group" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="enterprise_customers"
            disabled
          />
        </label>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Enterprise tier" />
        </label>
        <label>
          Description
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <GroupParentsField orgId={orgId} selfId={id} value={parents} onChange={setParents} />
        <FormActions cancelTo={resourcePath(orgId, 'customer-groups')} submitLabel="Save" />
      </form>
    </section>
  )
}
