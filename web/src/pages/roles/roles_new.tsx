import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createRole, listPermissions, type Permission } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function RolesNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [keys, setKeys] = useState<string[]>(['members:read'])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void listPermissions()
      .then((p) => setPermissions(p.items))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load permissions'))
  }, [])

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
    setKeys((prev) => (prev.includes(k) ? prev.filter((x) => x !== k) : [...prev, k]))
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const role = await createRole(orgId, {
        key,
        name,
        permission_keys: keys.length ? keys : ['members:read'],
      })
      navigate(resourcePath(orgId, 'roles', role.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Create role" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input value={key} onChange={(e) => setKey(e.target.value)} required />
        </label>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        {byCategory.map(([category, perms]) => (
          <fieldset key={category} className="perm-group">
            <legend>{category}</legend>
            {perms.map((p) => (
              <label key={p.key} className="perm">
                <input
                  type="checkbox"
                  checked={keys.includes(p.key)}
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
        <FormActions cancelTo={resourcePath(orgId, 'roles')} submitLabel="Create" />
      </form>
    </section>
  )
}
