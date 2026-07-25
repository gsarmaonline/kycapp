import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  getEmailTemplate,
  getOrganisation,
  type EmailTemplate,
  type Organisation,
} from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { emailRenderContext, renderEmailTemplate, wrapEmailHtml } from '../../email_render'
import { resourcePath } from '../../org_nav'

export function EmailTemplatesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<EmailTemplate | null>(null)
  const [org, setOrg] = useState<Organisation | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getEmailTemplate(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
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
          display_name: 'Pat',
          email: 'pat@example.com',
          attributes: { country: 'AU' },
        },
      }),
    [orgId, org?.name],
  )

  const previewHtml = useMemo(() => {
    if (!item) return ''
    return wrapEmailHtml(renderEmailTemplate(item.body_html, vars), {
      org_name: org?.name ?? 'Acme',
      logo_url: org?.logo_url,
      primary_color: org?.primary_color,
      accent_color: org?.accent_color,
      footer: org?.email_footer,
      font: org?.email_font,
      typography: org?.email_typography,
    })
  }, [item, vars, org])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Email template" />
      <DetailList
        items={[
          { label: 'Name', value: item.name },
          { label: 'Key', value: item.key },
          { label: 'System', value: item.is_system ? 'yes' : 'no' },
          { label: 'Status', value: item.status },
          { label: 'Subject', value: item.subject },
          { label: 'Body (text)', value: <pre className="preview-text">{item.body_text || '—'}</pre> },
        ]}
      />
      <fieldset className="perm-group email-preview">
        <legend>Preview (with branding)</legend>
        <p className="preview-subject">
          <span className="preview-label">Subject</span>
          {renderEmailTemplate(item.subject, vars)}
        </p>
        <pre className="preview-text">{renderEmailTemplate(item.body_text, vars)}</pre>
        <iframe title="HTML preview" className="preview-html" sandbox="" srcDoc={previewHtml} />
      </fieldset>
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'email-templates')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'email-templates', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
