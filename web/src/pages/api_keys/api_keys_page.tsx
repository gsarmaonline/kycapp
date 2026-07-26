import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  createOrgAPIKey,
  listOrgAPIKeys,
  listPermissions,
  revokeAPIKey,
  type OrgAPIKey,
  type Permission,
} from '../../api'
import { PageHeader, ResourceTable } from '../../crud/ui'
import { orgPath } from '../../org_nav'

function formatWhen(iso?: string) {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function scopesLabel(scopes: string[] | undefined) {
  if (!scopes || scopes.length === 0) return 'Full access'
  if (scopes.length <= 2) return scopes.join(', ')
  return `${scopes.slice(0, 2).join(', ')} +${scopes.length - 2}`
}

export function APIKeysPage() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<OrgAPIKey[]>([])
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [name, setName] = useState('Production')
  const [scopes, setScopes] = useState<string[]>([])
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const [keys, perms] = await Promise.all([listOrgAPIKeys(orgId), listPermissions()])
      setItems(keys.items)
      setPermissions(perms.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load API keys')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

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

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setMessage(null)
    setCreatedToken(null)
    try {
      const key = await createOrgAPIKey(orgId, {
        name,
        scopes: scopes.length ? scopes : undefined,
      })
      setCreatedToken(key.token ?? null)
      setName('Production')
      setScopes([])
      await refresh()
      setMessage('API key created — copy the token now; it will not be shown again')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create key failed')
    } finally {
      setBusy(false)
    }
  }

  async function onRevoke(k: OrgAPIKey) {
    if (!confirm(`Revoke API key “${k.name}”?`)) return
    setBusy(true)
    setError(null)
    setMessage(null)
    try {
      await revokeAPIKey(k.id)
      await refresh()
      setMessage('API key revoked')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Revoke failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section>
      <PageHeader title="API keys" />
      <p className="lede">
        Org-scoped keys for calling KYC from your product backend. Send{' '}
        <code>Authorization: Bearer kyc_…</code> on <code>/v1</code> requests. Requires the{' '}
        <code>api_access</code> entitlement. See the{' '}
        <Link to={orgPath(orgId, 'docs')}>Integration API</Link>.
      </p>
      {error && <p className="error">{error}</p>}
      {message && <p className="status">{message}</p>}
      {createdToken && (
        <p className="settings-token" role="status">
          <code>{createdToken}</code>
        </p>
      )}

      <form className="create stacked" onSubmit={onCreate}>
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
        <button type="submit" disabled={busy}>
          Create API key
        </button>
      </form>

      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Prefix', 'Scopes', 'Last used', 'Status', '']}
          empty="No API keys yet."
          rows={items.map((k) => ({
            key: k.id,
            cells: [
              k.name,
              <code key="prefix">{k.key_prefix}…</code>,
              scopesLabel(k.scopes),
              formatWhen(k.last_used_at),
              k.revoked ? 'Revoked' : 'Active',
              k.revoked ? (
                '—'
              ) : (
                <button
                  key="revoke"
                  type="button"
                  className="ghost"
                  disabled={busy}
                  onClick={() => void onRevoke(k)}
                >
                  Revoke
                </button>
              ),
            ],
          }))}
        />
      )}
    </section>
  )
}
