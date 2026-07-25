import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteOrganisationLogo,
  getOrganisation,
  updateOrganisation,
  uploadOrganisationLogo,
  type Organisation,
} from '../api'
import { ColorField, normalizeHex } from '../components/ColorField'
import { TypographyField } from '../components/TypographyField'
import { PageHeader } from '../crud/ui'
import { defaultTypography, resolveTypography, type EmailTypography } from '../email_fonts'
import { wrapEmailHtml } from '../email_render'

export function BrandingPage() {
  const { orgId = '' } = useParams()
  const [org, setOrg] = useState<Organisation | null>(null)
  const [primary, setPrimary] = useState('#1f4d3a')
  const [accent, setAccent] = useState('#16382a')
  const [footer, setFooter] = useState('')
  const [typography, setTypography] = useState<EmailTypography>(defaultTypography())
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      const o = await getOrganisation(orgId)
      setOrg(o)
      setPrimary(o.primary_color || '#1f4d3a')
      setAccent(o.accent_color || o.primary_color || '#16382a')
      setFooter(o.email_footer || '')
      setTypography(resolveTypography(o.email_typography, o.email_font || 'arial'))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load branding')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  const previewHtml = useMemo(() => {
    if (!org) return ''
    return wrapEmailHtml('<p>Hi {{display_name}},</p><p>This is how your emails will look.</p>', {
      org_name: org.name,
      logo_url: org.logo_url,
      primary_color: primary,
      accent_color: accent,
      footer,
      typography,
    }).replace(/\{\{\s*display_name\s*\}\}/g, 'Pat')
  }, [org, primary, accent, footer, typography])

  async function onSave(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError(null)
    try {
      const o = await updateOrganisation(orgId, {
        primary_color: normalizeHex(primary),
        accent_color: normalizeHex(accent),
        email_footer: footer,
        email_typography: typography,
      })
      setOrg(o)
      setPrimary(o.primary_color || normalizeHex(primary))
      setAccent(o.accent_color || normalizeHex(accent))
      setTypography(resolveTypography(o.email_typography, o.email_font || 'arial'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  async function onUpload(file: File | null) {
    if (!file) return
    setError(null)
    try {
      const o = await uploadOrganisationLogo(orgId, file)
      setOrg(o)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    }
  }

  async function onRemoveLogo() {
    setError(null)
    try {
      const o = await deleteOrganisationLogo(orgId)
      setOrg(o)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Remove failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Branding" />
      <p className="lede">
        Logo, colors, typography, and footer are applied to all email templates at preview (and send)
        time. Fonts are email-safe stacks so clients render them reliably.
      </p>
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSave}>
        <label>
          Logo
          <div className="branding-logo-row">
            {org?.logo_url ? (
              <img src={org.logo_url} alt="Organisation logo" className="branding-logo" />
            ) : (
              <span className="status">No logo uploaded</span>
            )}
            <input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              onChange={(e) => void onUpload(e.target.files?.[0] ?? null)}
            />
            {org?.logo_url ? (
              <button type="button" className="ghost" onClick={() => void onRemoveLogo()}>
                Remove logo
              </button>
            ) : null}
          </div>
        </label>
        <ColorField label="Primary color" value={primary} onChange={setPrimary} placeholder="#1f4d3a" />
        <ColorField label="Accent color" value={accent} onChange={setAccent} placeholder="#16382a" />
        <TypographyField
          label="Header typography"
          value={typography.header}
          onChange={(header) => setTypography({ ...typography, header })}
        />
        <TypographyField
          label="Body typography"
          value={typography.body}
          onChange={(body) => setTypography({ ...typography, body })}
        />
        <TypographyField
          label="Footer typography"
          value={typography.footer}
          onChange={(footerStyle) => setTypography({ ...typography, footer: footerStyle })}
        />
        <label>
          Email footer
          <textarea
            value={footer}
            onChange={(e) => setFooter(e.target.value)}
            rows={3}
            placeholder="© Acme · support@acme.com"
          />
        </label>
        <button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save branding'}
        </button>
      </form>

      <fieldset className="perm-group email-preview branding-email-preview">
        <legend>Email chrome preview</legend>
        <iframe title="Branding preview" className="preview-html" sandbox="" srcDoc={previewHtml} />
      </fieldset>
    </section>
  )
}
