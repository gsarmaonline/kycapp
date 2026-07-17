import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { inviteMember, listRoles, type Role } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function MembersNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [roles, setRoles] = useState<Role[]>([])
  const [email, setEmail] = useState('')
  const [roleId, setRoleId] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void listRoles(orgId)
      .then((r) => {
        setRoles(r.items)
        const member = r.items.find((x) => x.key === 'member')
        setRoleId(member?.id || r.items[0]?.id || '')
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load roles'))
  }, [orgId])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const m = await inviteMember(orgId, email, roleId)
      navigate(resourcePath(orgId, 'members', m.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invite failed')
    }
  }

  return (
    <section>
      <PageHeader title="Create member" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
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
        <FormActions cancelTo={resourcePath(orgId, 'members')} submitLabel="Invite" />
      </form>
    </section>
  )
}
