import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { getEmailTemplate, getOrganisation, updateEmailTemplate, type Organisation } from '../../api'
import { EmailBodySectionsEditor } from '../../components/EmailBodySectionsEditor'
import { FormActions, PageHeader } from '../../crud/ui'
import { VariableDocsHint } from '../../components/VariableDocsHint'
import {
  newBodySection,
  resolveTypography,
  sectionsFromLegacyHtml,
  type EmailBodySection,
} from '../../email_fonts'
import {
  composeBodySectionsHtml,
  emailRenderContext,
  renderEmailTemplate,
  resolveFrom,
  wrapEmailHtml,
} from '../../email_render'
import { orgPath, resourcePath } from '../../org_nav'

export function EmailTemplatesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [org, setOrg] = useState<Organisation | null>(null)
  const [name, setName] = useState('')
  const [subject, setSubject] = useState('')
  const [bodyText, setBodyText] = useState('')
  const [sections, setSections] = useState<EmailBodySection[]>([newBodySection()])
  const [fromName, setFromName] = useState('')
  const [fromAddress, setFromAddress] = useState('')
  const [sampleDisplayName, setSampleDisplayName] = useState('Pat')
  const [sampleEmail, setSampleEmail] = useState('pat@example.com')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void getEmailTemplate(id)
      .then((t) => {
        setName(t.name)
        setSubject(t.subject)
        setBodyText(t.body_text)
        setFromName(t.from_name || '')
        setFromAddress(t.from_address || '')
        if (t.body_sections && t.body_sections.length > 0) {
          setSections(t.body_sections)
        } else {
          setSections(sectionsFromLegacyHtml(t.body_html || t.body_text || ''))
        }
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => {
    void getOrganisation(orgId)
      .then(setOrg)
      .catch(() => undefined)
  }, [orgId])

  const vars = useMemo(
    () =>
      emailRenderContext({
        org_id: orgId,
        org_name: org?.name ?? 'Acme',
        app_user: {
          display_name: sampleDisplayName,
          email: sampleEmail,
          attributes: { country: 'AU' },
        },
      }),
    [orgId, org?.name, sampleDisplayName, sampleEmail],
  )

  const ty = useMemo(
    () => resolveTypography(org?.email_typography, org?.email_font || 'arial'),
    [org],
  )

  const previewHtml = useMemo(() => {
    const inner = composeBodySectionsHtml(sections, ty.body, vars)
    return wrapEmailHtml(inner, {
      org_name: org?.name ?? 'Acme',
      logo_url: org?.logo_url,
      primary_color: org?.primary_color,
      accent_color: org?.accent_color,
      footer: org?.email_footer,
      font: org?.email_font,
      typography: org?.email_typography,
    })
  }, [sections, vars, org, ty.body])

  const previewFrom = useMemo(
    () =>
      resolveFrom(
        fromName,
        fromAddress,
        org?.email_from_name || '',
        org?.email_from_address || '',
        'EMAIL_FROM',
      ),
    [fromName, fromAddress, org],
  )

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateEmailTemplate(id, {
        name,
        subject,
        body_text: bodyText,
        body_sections: sections,
        from_name: fromName,
        from_address: fromAddress,
      })
      navigate(resourcePath(orgId, 'email-templates', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit email template" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Subject
          <VariableDocsHint>
            Placeholders e.g. <code>{'{{app_user.display_name}}'}</code>,{' '}
            <code>{'{{organisation.name}}'}</code>.
          </VariableDocsHint>
          <input value={subject} onChange={(e) => setSubject(e.target.value)} required />
        </label>
        <fieldset className="settings-block">
          <legend>From (optional override)</legend>
          <p className="field-hint">
            Leave blank to use the org default from{' '}
            <Link to={orgPath(orgId, 'branding')}>Branding</Link>.
          </p>
          <label>
            From name
            <input value={fromName} onChange={(e) => setFromName(e.target.value)} placeholder="Acme" />
          </label>
          <label>
            From address
            <input
              value={fromAddress}
              onChange={(e) => setFromAddress(e.target.value)}
              placeholder="hello@acme.com"
            />
          </label>
        </fieldset>
        <label>
          Body (text)
          <textarea value={bodyText} onChange={(e) => setBodyText(e.target.value)} rows={4} />
        </label>
        <EmailBodySectionsEditor sections={sections} onChange={setSections} brandingHint />
        <FormActions cancelTo={resourcePath(orgId, 'email-templates', id)} submitLabel="Save" />
      </form>

      <fieldset className="perm-group email-preview">
        <legend>Preview</legend>
        <p className="field-hint">From: {previewFrom}</p>
        <div className="create stacked preview-vars">
          <label>
            Sample display name
            <input value={sampleDisplayName} onChange={(e) => setSampleDisplayName(e.target.value)} />
          </label>
          <label>
            Sample email
            <input value={sampleEmail} onChange={(e) => setSampleEmail(e.target.value)} />
          </label>
        </div>
        <p className="preview-subject">
          <span className="preview-label">Subject</span>
          {renderEmailTemplate(subject, vars)}
        </p>
        <iframe title="HTML preview" className="preview-html" sandbox="" srcDoc={previewHtml} />
      </fieldset>
    </section>
  )
}
