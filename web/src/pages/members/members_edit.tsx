import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getMembership, listRoles, updateMembership, type Role } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function MembersEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [roles, setRoles] = useState<Role[]>([])
  const [roleId, setRoleId] = useState('')
  const [status, setStatus] = useState('active')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void (async () => {
      try {
        const [m, r] = await Promise.all([getMembership(id), listRoles(orgId)])
        setRoles(r.items)
        setRoleId(m.role_id)
        setStatus(m.status)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      } finally {
        setLoading(false)
      }
    })()
  }, [id, orgId])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateMembership(id, { role_id: roleId, status })
      navigate(resourcePath(orgId, 'members', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit member" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Role
          <select value={roleId} onChange={(e) => setRoleId(e.target.value)} required>
            {roles.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="invited">invited</option>
            <option value="active">active</option>
            <option value="revoked">revoked</option>
          </select>
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'members', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
