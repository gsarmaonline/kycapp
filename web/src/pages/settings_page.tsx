import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  createOrgAPIKey,
  deleteOrganisation,
  deleteOrgIntegration,
  getOrganisation,
  importStripeCatalog,
  listOrgAPIKeys,
  listOrgIntegrations,
  listStripeCatalog,
  revokeAPIKey,
  syncProductPlansToStripe,
  updateOrganisation,
  upsertStripeIntegration,
  type OrgAPIKey,
  type OrgIntegration,
  type Organisation,
  type StripeCatalogItem,
} from '../api'
import { PageHeader } from '../crud/ui'
import { resourcePath } from '../org_nav'

export function SettingsPage() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [org, setOrg] = useState<Organisation | null>(null)
  const [name, setName] = useState('')
  const [integrations, setIntegrations] = useState<OrgIntegration[]>([])
  const [apiKeys, setApiKeys] = useState<OrgAPIKey[]>([])
  const [stripeSecret, setStripeSecret] = useState('')
  const [stripePublishable, setStripePublishable] = useState('')
  const [catalog, setCatalog] = useState<StripeCatalogItem[] | null>(null)
  const [selectedPrices, setSelectedPrices] = useState<Record<string, boolean>>({})
  const [newKeyName, setNewKeyName] = useState('Default')
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  const stripe = integrations.find((i) => i.provider === 'stripe')

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const [o, ints, keys] = await Promise.all([
        getOrganisation(orgId),
        listOrgIntegrations(orgId),
        listOrgAPIKeys(orgId),
      ])
      setOrg(o)
      setName(o.name)
      setIntegrations(ints.items)
      setApiKeys(keys.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load settings')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onSaveName(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setMessage(null)
    try {
      const o = await updateOrganisation(orgId, { name })
      setOrg(o)
      setMessage('Organisation name saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setBusy(false)
    }
  }

  async function afterStripeConnected() {
    try {
      const pushed = await syncProductPlansToStripe(orgId)
      const remote = await listStripeCatalog(orgId)
      setCatalog(remote.items)
      const sel: Record<string, boolean> = {}
      for (const item of remote.items) sel[item.price_ref] = true
      setSelectedPrices(sel)
      const parts = [`Stripe connected`]
      if (pushed.pushed.length) {
        parts.push(`pushed ${pushed.pushed.length} local plan price(s)`)
      }
      if (remote.items.length) {
        parts.push(`found ${remote.items.length} Stripe price(s) — import below if you want`)
      }
      setMessage(parts.join(' · '))
    } catch (err) {
      setMessage(
        err instanceof Error
          ? `Stripe connected · catalog: ${err.message}`
          : 'Stripe connected · catalog unavailable',
      )
    }
  }

  async function onSaveStripe(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setMessage(null)
    try {
      const row = await upsertStripeIntegration(orgId, {
        secret_key: stripeSecret || undefined,
        publishable_key: stripePublishable || undefined,
      })
      setIntegrations((prev) => {
        const rest = prev.filter((i) => i.provider !== 'stripe')
        return [...rest, row]
      })
      setStripeSecret('')
      setStripePublishable('')
      await afterStripeConnected()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Stripe save failed')
    } finally {
      setBusy(false)
    }
  }

  async function onDisconnectStripe() {
    if (!confirm('Disconnect Stripe from this organisation?')) return
    setBusy(true)
    setError(null)
    try {
      await deleteOrgIntegration(orgId, 'stripe')
      setIntegrations((prev) => prev.filter((i) => i.provider !== 'stripe'))
      setCatalog(null)
      setSelectedPrices({})
      setMessage('Stripe disconnected')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Disconnect failed')
    } finally {
      setBusy(false)
    }
  }

  async function onLoadCatalog() {
    setBusy(true)
    setError(null)
    try {
      const remote = await listStripeCatalog(orgId)
      setCatalog(remote.items)
      const sel: Record<string, boolean> = {}
      for (const item of remote.items) sel[item.price_ref] = true
      setSelectedPrices(sel)
      setMessage(`Loaded ${remote.items.length} Stripe price(s)`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Catalog load failed')
    } finally {
      setBusy(false)
    }
  }

  async function onImportSelected() {
    const items = Object.entries(selectedPrices)
      .filter(([, on]) => on)
      .map(([price_ref]) => ({ price_ref }))
    if (!items.length) {
      setError('Select at least one Stripe price to import')
      return
    }
    setBusy(true)
    setError(null)
    try {
      const result = await importStripeCatalog(orgId, items)
      setMessage(
        `Imported ${result.imported.length} plan(s)` +
          (result.skipped ? ` · skipped ${result.skipped} already linked` : ''),
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Import failed')
    } finally {
      setBusy(false)
    }
  }

  async function onSyncOut() {
    setBusy(true)
    setError(null)
    try {
      const result = await syncProductPlansToStripe(orgId)
      setMessage(
        result.pushed.length
          ? `Pushed ${result.pushed.length} plan price(s) to Stripe`
          : 'All local plan prices already synced',
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sync failed')
    } finally {
      setBusy(false)
    }
  }

  async function onCreateKey(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    setMessage(null)
    setCreatedToken(null)
    try {
      const key = await createOrgAPIKey(orgId, newKeyName)
      setCreatedToken(key.token ?? null)
      setNewKeyName('Default')
      await refresh()
      setMessage('API key created — copy the token now; it will not be shown again')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create key failed')
    } finally {
      setBusy(false)
    }
  }

  async function onRevokeKey(id: string) {
    if (!confirm('Revoke this API key?')) return
    setBusy(true)
    setError(null)
    try {
      await revokeAPIKey(id)
      await refresh()
      setMessage('API key revoked')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Revoke failed')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete() {
    if (!org) return
    const typed = window.prompt(
      `Type ${org.slug} to permanently delete this organisation and all of its data`,
    )
    if (typed !== org.slug) {
      if (typed != null) setError('Slug did not match — organisation not deleted')
      return
    }
    setBusy(true)
    setError(null)
    try {
      await deleteOrganisation(orgId)
      navigate('/app', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
      setBusy(false)
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section className="settings">
      <PageHeader title="Settings" />
      <p className="lede">Organisation profile, integrations, API keys, and danger zone.</p>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {message && <p className="status">{message}</p>}

      <section className="settings-block">
        <h3>Organisation</h3>
        <form className="create stacked" onSubmit={onSaveName}>
          <label>
            Name
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </label>
          <p className="status">Slug: {org?.slug}</p>
          <button type="submit" disabled={busy}>
            Save name
          </button>
        </form>
      </section>

      <section className="settings-block">
        <h3>Stripe</h3>
        <p className="status">
          Connect your Stripe keys so KYC can sync product plan prices. KYC remains source of truth
          for features; Stripe holds the charge objects. Keys are never shown in full after save.
        </p>
        {stripe?.has_secret ? (
          <p className="status">
            Connected · secret {stripe.secret_hint}
            {stripe.has_public_key ? ` · publishable ${stripe.public_key_hint}` : ''}
          </p>
        ) : (
          <p className="status">Not connected</p>
        )}
        <form className="create stacked" onSubmit={onSaveStripe}>
          <label>
            Secret key
            <input
              value={stripeSecret}
              onChange={(e) => setStripeSecret(e.target.value)}
              placeholder={stripe?.has_secret ? 'Leave blank to keep current' : 'sk_live_…'}
              autoComplete="off"
            />
          </label>
          <label>
            Publishable key
            <input
              value={stripePublishable}
              onChange={(e) => setStripePublishable(e.target.value)}
              placeholder={stripe?.has_public_key ? 'Leave blank to keep current' : 'pk_live_…'}
              autoComplete="off"
            />
          </label>
          <div className="row" style={{ gap: '0.75rem', flexWrap: 'wrap' }}>
            <button type="submit" disabled={busy}>
              {stripe?.has_secret ? 'Update Stripe' : 'Connect Stripe'}
            </button>
            {stripe?.has_secret && (
              <>
                <button type="button" className="ghost" disabled={busy} onClick={() => void onLoadCatalog()}>
                  Load catalog
                </button>
                <button type="button" className="ghost" disabled={busy} onClick={() => void onSyncOut()}>
                  Push KYC plans
                </button>
                <button type="button" className="ghost" disabled={busy} onClick={() => void onDisconnectStripe()}>
                  Disconnect
                </button>
              </>
            )}
          </div>
        </form>
        {catalog && (
          <div className="create stacked" style={{ marginTop: '1rem' }}>
            <h4>Import from Stripe</h4>
            <p className="status">
              Creates product plans linked to these Prices. Attach features afterward in{' '}
              <Link to={resourcePath(orgId, 'product-plans')}>Product plans</Link>.
            </p>
            {catalog.length === 0 ? (
              <p className="status">No active recurring prices found.</p>
            ) : (
              catalog.map((item) => (
                <label key={item.price_ref} className="perm">
                  <input
                    type="checkbox"
                    checked={!!selectedPrices[item.price_ref]}
                    onChange={(e) =>
                      setSelectedPrices((prev) => ({ ...prev, [item.price_ref]: e.target.checked }))
                    }
                  />
                  <span>
                    {item.product_name || item.product_ref} ·{' '}
                    {(item.unit_amount / 100).toFixed(2)} {item.currency.toUpperCase()}/
                    {item.interval} · <code>{item.price_ref}</code>
                  </span>
                </label>
              ))
            )}
            {catalog.length > 0 && (
              <button type="button" disabled={busy} onClick={() => void onImportSelected()}>
                Import selected
              </button>
            )}
          </div>
        )}
      </section>

      <section className="settings-block">
        <h3>KYC API keys</h3>
        <p className="status">
          Org-scoped keys for calling KYC from your product backend. Shown once at creation.
        </p>
        {createdToken && (
          <p className="settings-token" role="status">
            <code>{createdToken}</code>
          </p>
        )}
        <form className="create stacked" onSubmit={onCreateKey}>
          <label>
            Name
            <input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} required />
          </label>
          <button type="submit" disabled={busy}>
            Create API key
          </button>
        </form>
        <ul className="settings-key-list">
          {apiKeys.length === 0 && <li className="status">No API keys yet</li>}
          {apiKeys.map((k) => (
            <li key={k.id}>
              <span>
                <strong>{k.name}</strong> · <code>{k.key_prefix}…</code>
                {k.revoked ? ' · revoked' : ''}
              </span>
              {!k.revoked && (
                <button type="button" className="ghost" disabled={busy} onClick={() => void onRevokeKey(k.id)}>
                  Revoke
                </button>
              )}
            </li>
          ))}
        </ul>
      </section>

      <section className="settings-block settings-danger">
        <h3>Delete organisation</h3>
        <p className="status">
          Permanently deletes this organisation and all related data (members, users, plans,
          automations, API keys). This cannot be undone.
        </p>
        <button type="button" className="danger" disabled={busy} onClick={() => void onDelete()}>
          Delete organisation
        </button>
      </section>
    </section>
  )
}
