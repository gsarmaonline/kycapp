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
  const [fromName, setFromName] = useState('')
  const [fromAddress, setFromAddress] = useState('')
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
      setFromName(o.email_from_name || '')
      setFromAddress(o.email_from_address || '')
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
    return wrapEmailHtml(
      '<p>Hi {{app_user.display_name}},</p><p>This is how your emails will look.</p>'.replace(
        /\{\{\s*app_user\.display_name\s*\}\}/g,
        'Pat',
      ),
      {
        org_name: org.name,
        logo_url: org.logo_url,
        primary_color: primary,
        accent_color: accent,
        footer,
        typography,
      },
    )
  }, [org, primary, accent, footer, typography])

  const previewFrom = useMemo(() => {
    const name = fromName.trim()
    const addr = fromAddress.trim()
    if (addr) return name ? `${name} <${addr}>` : addr
    return 'Uses deployment EMAIL_FROM'
  }, [fromName, fromAddress])

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
        email_from_name: fromName,
        email_from_address: fromAddress,
      })
      setOrg(o)
      setPrimary(o.primary_color || normalizeHex(primary))
      setAccent(o.accent_color || normalizeHex(accent))
      setFromName(o.email_from_name || '')
      setFromAddress(o.email_from_address || '')
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
        Branding is the style base for all emails: header, default body block style, footer, and
        default From. Each email template owns its body sections and can override From.
      </p>
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSave}>
        <fieldset className="settings-block">
          <legend>Header</legend>
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
            label="Header style"
            value={typography.header}
            onChange={(header) =>
              setTypography({ ...typography, header: { ...typography.header, ...header } as EmailTypography['header'] })
            }
          />
        </fieldset>

        <fieldset className="settings-block">
          <legend>Body default</legend>
          <p className="field-hint">
            Default style for each email template body section. Templates can override per section.
          </p>
          <TypographyField
            label="Body style"
            value={typography.body}
            onChange={(body) =>
              setTypography({ ...typography, body: { ...typography.body, ...body } as EmailTypography['body'] })
            }
          />
        </fieldset>

        <fieldset className="settings-block">
          <legend>Footer</legend>
          <label>
            Email footer
            <textarea
              value={footer}
              onChange={(e) => setFooter(e.target.value)}
              rows={3}
              placeholder="© Acme · support@acme.com"
            />
          </label>
          <TypographyField
            label="Footer style"
            value={typography.footer}
            onChange={(footerStyle) =>
              setTypography({
                ...typography,
                footer: { ...typography.footer, ...footerStyle } as EmailTypography['footer'],
              })
            }
          />
        </fieldset>

        <fieldset className="settings-block">
          <legend>Default From</legend>
          <p className="field-hint">
            Used when a template does not set its own From. Address must be verified with your email
            provider (e.g. Resend). Leave blank to use the deployment EMAIL_FROM.
          </p>
          <label>
            From name
            <input
              value={fromName}
              onChange={(e) => setFromName(e.target.value)}
              placeholder="Acme Support"
            />
          </label>
          <label>
            From address
            <input
              value={fromAddress}
              onChange={(e) => setFromAddress(e.target.value)}
              placeholder="support@acme.com"
            />
          </label>
        </fieldset>

        <button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save branding'}
        </button>
      </form>

      <fieldset className="perm-group email-preview branding-email-preview">
        <legend>Email chrome preview</legend>
        <p className="field-hint">From: {previewFrom}</p>
        <iframe title="Branding preview" className="preview-html" sandbox="" srcDoc={previewHtml} />
      </fieldset>
    </section>
  )
}
