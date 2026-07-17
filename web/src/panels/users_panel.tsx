import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import {
  createAppUser,
  listAppUsers,
  listAttributeDefinitions,
  type AppUser,
  type AttributeDefinition,
} from '../api'
import { groupDefsBySection } from './attributes_panel'

export function UsersPanel({
  orgId,
  onError,
}: {
  orgId: string
  onError: (msg: string | null) => void
}) {
  const [users, setUsers] = useState<AppUser[]>([])
  const [defs, setDefs] = useState<AttributeDefinition[]>([])
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [attrDraft, setAttrDraft] = useState<Record<string, string>>({})

  async function refresh() {
    onError(null)
    try {
      const [u, d] = await Promise.all([
        listAppUsers(orgId),
        listAttributeDefinitions(orgId, 'active'),
      ])
      setUsers(u.items)
      setDefs(d.items)
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Users load failed')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  function coerceAttributes(): Record<string, unknown> {
    const out: Record<string, unknown> = {}
    for (const d of defs) {
      const raw = attrDraft[d.key]
      if (raw === undefined || raw === '') continue
      switch (d.value_type) {
        case 'number':
          out[d.key] = Number(raw)
          break
        case 'boolean':
          out[d.key] = raw === 'true'
          break
        default:
          out[d.key] = raw
      }
    }
    return out
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      await createAppUser(orgId, {
        email: email || undefined,
        display_name: displayName,
        attributes: coerceAttributes(),
      })
      setEmail('')
      setDisplayName('')
      setAttrDraft({})
      await refresh()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Create user failed')
    }
  }

  const grouped = groupDefsBySection(defs)

  return (
    <section className="app-users">
      <p className="lede">
        End users of your product (not team members). Profile fields come from User Attributes.
      </p>
      <form className="create stacked" onSubmit={onCreate}>
        <label>
          Display name
          <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </label>
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </label>
        {grouped.map(([sec, items]) => (
          <fieldset key={sec} className="perm-group attr-section">
            <legend>{sec}</legend>
            {items.map((d) => (
              <label key={d.id}>
                {d.label}
                {d.required ? ' *' : ''}
                {d.value_type === 'boolean' ? (
                  <select
                    value={attrDraft[d.key] ?? ''}
                    onChange={(e) =>
                      setAttrDraft((prev) => ({ ...prev, [d.key]: e.target.value }))
                    }
                  >
                    <option value="">—</option>
                    <option value="true">true</option>
                    <option value="false">false</option>
                  </select>
                ) : d.value_type === 'dropdown' ? (
                  <select
                    value={attrDraft[d.key] ?? ''}
                    onChange={(e) =>
                      setAttrDraft((prev) => ({ ...prev, [d.key]: e.target.value }))
                    }
                  >
                    <option value="">—</option>
                    {d.enum_values.map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type={d.value_type === 'number' ? 'number' : d.value_type === 'date' ? 'date' : 'text'}
                    value={attrDraft[d.key] ?? ''}
                    onChange={(e) =>
                      setAttrDraft((prev) => ({ ...prev, [d.key]: e.target.value }))
                    }
                    required={d.required}
                  />
                )}
              </label>
            ))}
          </fieldset>
        ))}
        <button type="submit">Create user</button>
      </form>

      <ul className="list">
        {users.map((u) => (
          <li key={u.id} className="member">
            <strong>{u.display_name || u.email || u.id}</strong>
            <span>{u.email || '—'}</span>
            <span className="status">{u.status}</span>
            <span className="attr-preview">
              {Object.keys(u.attributes || {}).length
                ? Object.entries(u.attributes)
                    .map(([k, v]) => `${k}=${String(v)}`)
                    .join(', ')
                : 'no attributes'}
            </span>
          </li>
        ))}
        {users.length === 0 && <li className="empty">No users yet.</li>}
      </ul>
    </section>
  )
}
