import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { createRole, updateRole, type Permission, type Role } from '../api'

export function RolesPanel({
  orgId,
  roles,
  permissions,
  onChanged,
  onError,
}: {
  orgId: string
  roles: Role[]
  permissions: Permission[]
  onChanged: () => Promise<void>
  onError: (msg: string | null) => void
}) {
  const [selectedId, setSelectedId] = useState('')
  const [draftKeys, setDraftKeys] = useState<string[]>([])
  const [newKey, setNewKey] = useState('')
  const [newName, setNewName] = useState('')

  const selected = roles.find((r) => r.id === selectedId) ?? roles[0]
  const locked = selected?.key === 'owner'

  useEffect(() => {
    if (!selectedId && roles[0]) {
      setSelectedId(roles[0].id)
    }
  }, [roles, selectedId])

  useEffect(() => {
    if (selected) {
      setDraftKeys(selected.permission_keys ?? [])
    }
  }, [selected?.id, selected?.permission_keys])

  const byCategory = useMemo(() => {
    const map = new Map<string, Permission[]>()
    for (const p of permissions) {
      const list = map.get(p.category) ?? []
      list.push(p)
      map.set(p.category, list)
    }
    return [...map.entries()]
  }, [permissions])

  function toggle(key: string) {
    if (locked) return
    setDraftKeys((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key],
    )
  }

  async function onSave(e: FormEvent) {
    e.preventDefault()
    if (!selected || locked) return
    onError(null)
    try {
      await updateRole(selected.id, draftKeys)
      await onChanged()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Save role failed')
    }
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      const role = await createRole(orgId, {
        key: newKey,
        name: newName,
        permission_keys: draftKeys.length ? draftKeys : ['members:read'],
      })
      setNewKey('')
      setNewName('')
      await onChanged()
      setSelectedId(role.id)
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Create role failed')
    }
  }

  return (
    <section className="roles">
      <form className="create" onSubmit={onCreate}>
        <label>
          New role key
          <input value={newKey} onChange={(e) => setNewKey(e.target.value)} required />
        </label>
        <label>
          Name
          <input value={newName} onChange={(e) => setNewName(e.target.value)} required />
        </label>
        <button type="submit">Create role</button>
      </form>

      <div className="role-toolbar">
        <label>
          Edit role
          <select value={selected?.id ?? ''} onChange={(e) => setSelectedId(e.target.value)}>
            {roles.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name} ({r.key})
              </option>
            ))}
          </select>
        </label>
        {locked && <p className="status">Owner permissions are locked.</p>}
      </div>

      <form onSubmit={onSave}>
        {byCategory.map(([category, perms]) => (
          <fieldset key={category} className="perm-group">
            <legend>{category}</legend>
            {perms.map((p) => (
              <label key={p.key} className="perm">
                <input
                  type="checkbox"
                  checked={draftKeys.includes(p.key)}
                  disabled={locked}
                  onChange={() => toggle(p.key)}
                />
                <span>
                  <strong>{p.key}</strong>
                  <em>{p.description}</em>
                </span>
              </label>
            ))}
          </fieldset>
        ))}
        <button type="submit" disabled={locked || !selected}>
          Save permissions
        </button>
      </form>
    </section>
  )
}
