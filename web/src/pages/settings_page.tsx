import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  archiveOrganisation,
  createOrgAPIKey,
  deleteOrgIntegration,
  getOrganisation,
  listOrgAPIKeys,
  listOrgIntegrations,
  revokeAPIKey,
  updateOrganisation,
  upsertStripeIntegration,
  type OrgAPIKey,
  type OrgIntegration,
  type Organisation,
} from '../api'
import { PageHeader } from '../crud/ui'

export function SettingsPage() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [org, setOrg] = useState<Organisation | null>(null)
  const [name, setName] = useState('')
  const [integrations, setIntegrations] = useState<OrgIntegration[]>([])
  const [apiKeys, setApiKeys] = useState<OrgAPIKey[]>([])
  const [stripeSecret, setStripeSecret] = useState('')
  const [stripePublishable, setStripePublishable] = useState('')
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
      setMessage('Stripe connected')
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
      setMessage('Stripe disconnected')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Disconnect failed')
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

  async function onArchive() {
    if (!org) return
    const typed = window.prompt(`Type ${org.slug} to delete (archive) this organisation`)
    if (typed !== org.slug) {
      if (typed != null) setError('Slug did not match — organisation not deleted')
      return
    }
    setBusy(true)
    setError(null)
    try {
      await archiveOrganisation(orgId)
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
          Connect your Stripe keys so KYC can run billing for your product. Keys are stored on this
          organisation and never shown in full after save.
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
              <button type="button" className="ghost" disabled={busy} onClick={() => void onDisconnectStripe()}>
                Disconnect
              </button>
            )}
          </div>
        </form>
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
          Archives this organisation. Members lose access; data is retained for recovery. This cannot
          be undone from the UI.
        </p>
        <button type="button" className="danger" disabled={busy} onClick={() => void onArchive()}>
          Delete organisation
        </button>
      </section>
    </section>
  )
}
