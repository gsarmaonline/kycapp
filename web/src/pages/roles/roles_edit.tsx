import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getRole, listPermissions, updateRole, type Permission } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function RolesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [keys, setKeys] = useState<string[]>([])
  const [locked, setLocked] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void (async () => {
      try {
        const [role, perms] = await Promise.all([getRole(id), listPermissions()])
        setPermissions(perms.items)
        setName(role.name)
        setDescription(role.description || '')
        setKeys(role.permission_keys ?? [])
        setLocked(role.key === 'owner')
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      } finally {
        setLoading(false)
      }
    })()
  }, [id])

  const byCategory = useMemo(() => {
    const map = new Map<string, Permission[]>()
    for (const p of permissions) {
      const list = map.get(p.category) ?? []
      list.push(p)
      map.set(p.category, list)
    }
    return [...map.entries()]
  }, [permissions])

  function toggle(k: string) {
    if (locked) return
    setKeys((prev) => (prev.includes(k) ? prev.filter((x) => x !== k) : [...prev, k]))
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (locked) return
    setError(null)
    try {
      await updateRole(id, { name, description, permission_keys: keys })
      navigate(resourcePath(orgId, 'roles', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit role" />
      {locked && <p className="status">Owner permissions are locked.</p>}
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required disabled={locked} />
        </label>
        <label>
          Description
          <input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={locked}
          />
        </label>
        {byCategory.map(([category, perms]) => (
          <fieldset key={category} className="perm-group">
            <legend>{category}</legend>
            {perms.map((p) => (
              <label key={p.key} className="perm">
                <input
                  type="checkbox"
                  checked={keys.includes(p.key)}
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
        <FormActions cancelTo={resourcePath(orgId, 'roles', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
