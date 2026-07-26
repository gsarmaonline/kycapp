import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { listOrgAPIKeys, revokeAPIKey, type OrgAPIKey } from '../../api'
import { PageHeader, ResourceTable } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'

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

export function APIKeysIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<OrgAPIKey[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listOrgAPIKeys(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load API keys')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onRevoke(k: OrgAPIKey) {
    if (!confirm(`Revoke API key “${k.name}”?`)) return
    setBusy(true)
    setError(null)
    try {
      await revokeAPIKey(k.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Revoke failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section>
      <PageHeader
        title="API keys"
        createTo={resourcePath(orgId, 'api-keys', 'new')}
        createLabel="Create API key"
      />
      <p className="lede">
        Org-scoped keys for calling KYC from your product backend. Send{' '}
        <code>Authorization: Bearer kyc_…</code> on <code>/v1</code> requests. Requires the{' '}
        <code>api_access</code> entitlement. See the{' '}
        <Link to={orgPath(orgId, 'docs')}>Integration API</Link>.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Prefix', 'Scopes', 'Last used', 'Status']}
          empty="No API keys yet."
          rows={items.map((k) => ({
            key: k.id,
            cells: [
              k.name,
              <code key="prefix">{k.key_prefix}…</code>,
              scopesLabel(k.scopes),
              formatWhen(k.last_used_at),
              k.revoked ? 'Revoked' : 'Active',
            ],
            actions: k.revoked ? null : (
              <button
                type="button"
                className="link-btn danger"
                disabled={busy}
                onClick={() => void onRevoke(k)}
              >
                Revoke
              </button>
            ),
          }))}
        />
      )}
    </section>
  )
}
