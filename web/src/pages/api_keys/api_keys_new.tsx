import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { createOrgAPIKey, listPermissions, type Permission } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function APIKeysNew() {
  const { orgId = '' } = useParams()
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [name, setName] = useState('Production')
  const [scopes, setScopes] = useState<string[]>([])
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const perms = await listPermissions()
        if (!cancelled) setPermissions(perms.items)
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : 'Failed to load permissions')
        }
      }
    })()
    return () => {
      cancelled = true
    }
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

  function toggleScope(key: string) {
    setScopes((prev) => (prev.includes(key) ? prev.filter((x) => x !== key) : [...prev, key]))
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    setError(null)
    setCreatedToken(null)
    try {
      const key = await createOrgAPIKey(orgId, {
        name,
        scopes: scopes.length ? scopes : undefined,
      })
      setCreatedToken(key.token ?? null)
      setName('Production')
      setScopes([])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create key failed')
    } finally {
      setBusy(false)
    }
  }

  if (createdToken) {
    return (
      <section>
        <PageHeader title="API key created" />
        <p className="status" role="status">
          Copy the token now — it will not be shown again.
        </p>
        <p className="settings-token" role="status">
          <code>{createdToken}</code>
        </p>
        <p>
          <Link className="button" to={resourcePath(orgId, 'api-keys')}>
            Back to API keys
          </Link>
        </p>
      </section>
    )
  }

  return (
    <section>
      <PageHeader title="Create API key" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={(e) => void onSubmit(e)}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <p className="field-hint">
          Scopes limit the key to selected permissions. Leave all unchecked for full organisation
          access.
        </p>
        {byCategory.map(([category, perms]) => (
          <fieldset key={category} className="perm-group">
            <legend>{category}</legend>
            {perms.map((p) => (
              <label key={p.key} className="perm">
                <input
                  type="checkbox"
                  checked={scopes.includes(p.key)}
                  onChange={() => toggleScope(p.key)}
                />
                <span>
                  <strong>{p.key}</strong>
                  <em>{p.description}</em>
                </span>
              </label>
            ))}
          </fieldset>
        ))}
        <FormActions
          cancelTo={resourcePath(orgId, 'api-keys')}
          submitLabel={busy ? 'Creating…' : 'Create API key'}
        />
      </form>
    </section>
  )
}
