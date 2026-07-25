import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getEmailTemplate, getOrganisation, updateEmailTemplate, type Organisation } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { VariableDocsHint } from '../../components/VariableDocsHint'
import { emailRenderContext, renderEmailTemplate, wrapEmailHtml } from '../../email_render'
import { resourcePath } from '../../org_nav'

export function EmailTemplatesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [org, setOrg] = useState<Organisation | null>(null)
  const [name, setName] = useState('')
  const [subject, setSubject] = useState('')
  const [bodyText, setBodyText] = useState('')
  const [bodyHtml, setBodyHtml] = useState('')
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
        setBodyHtml(t.body_html)
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

  const previewHtml = useMemo(() => {
    const rendered = renderEmailTemplate(bodyHtml, vars)
    return wrapEmailHtml(rendered, {
      org_name: org?.name ?? 'Acme',
      logo_url: org?.logo_url,
      primary_color: org?.primary_color,
      accent_color: org?.accent_color,
      footer: org?.email_footer,
      font: org?.email_font,
      typography: org?.email_typography,
    })
  }, [bodyHtml, vars, org])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateEmailTemplate(id, {
        name,
        subject,
        body_text: bodyText,
        body_html: bodyHtml,
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
        <label>
          Body (text)
          <textarea value={bodyText} onChange={(e) => setBodyText(e.target.value)} rows={6} />
        </label>
        <label>
          Body (HTML)
          <VariableDocsHint>
            Inner content only — header, logo, and footer come from Branding.
          </VariableDocsHint>
          <textarea value={bodyHtml} onChange={(e) => setBodyHtml(e.target.value)} rows={6} />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'email-templates', id)} submitLabel="Save" />
      </form>

      <fieldset className="perm-group email-preview">
        <legend>Preview</legend>
        <div className="create stacked preview-vars">
          <label>
            Sample {'{{app_user.display_name}}'}
            <input
              value={sampleDisplayName}
              onChange={(e) => setSampleDisplayName(e.target.value)}
            />
          </label>
          <label>
            Sample {'{{app_user.email}}'}
            <input value={sampleEmail} onChange={(e) => setSampleEmail(e.target.value)} />
          </label>
        </div>
        <p className="preview-subject">
          <span className="preview-label">Subject</span>
          {renderEmailTemplate(subject, vars)}
        </p>
        <pre className="preview-text">{renderEmailTemplate(bodyText, vars) || '—'}</pre>
        <iframe title="HTML preview" className="preview-html" sandbox="" srcDoc={previewHtml} />
      </fieldset>
    </section>
  )
}
