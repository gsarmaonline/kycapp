import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { inviteMember, type Membership, type Role } from '../api'

export function MembersPanel({
  orgId,
  members,
  roles,
  onChanged,
  onError,
}: {
  orgId: string
  members: Membership[]
  roles: Role[]
  onChanged: () => Promise<void>
  onError: (msg: string | null) => void
}) {
  const [email, setEmail] = useState('')
  const [roleId, setRoleId] = useState('')

  useEffect(() => {
    const memberRole = roles.find((x) => x.key === 'member')
    setRoleId((prev) => prev || memberRole?.id || roles[0]?.id || '')
  }, [roles])

  async function onInvite(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      await inviteMember(orgId, email, roleId)
      setEmail('')
      await onChanged()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Invite failed')
    }
  }

  return (
    <section>
      <form className="create" onSubmit={onInvite}>
        <label>
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
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
        <button type="submit">Invite</button>
      </form>

      <ul className="list">
        {members.map((m) => (
          <li key={m.id} className="member">
            <strong>{m.user_name || m.user_email}</strong>
            <span>{m.user_email}</span>
            <span>{m.role_key}</span>
            <span className="status">{m.status}</span>
          </li>
        ))}
        {members.length === 0 && <li className="empty">No members yet.</li>}
      </ul>
    </section>
  )
}
