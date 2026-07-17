import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getEmailTemplate, getOrganisation, type EmailTemplate } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { renderEmailTemplate } from '../../email_render'
import { resourcePath } from '../../org_nav'

export function EmailTemplatesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<EmailTemplate | null>(null)
  const [orgName, setOrgName] = useState('Acme')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getEmailTemplate(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [id])

  useEffect(() => {
    void getOrganisation(orgId)
      .then((o) => setOrgName(o.name))
      .catch(() => undefined)
  }, [orgId])

  const vars = useMemo(
    () => ({ display_name: 'Pat', org_name: orgName }),
    [orgName],
  )

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
        <legend>Preview</legend>
        <p className="preview-subject">
          <span className="preview-label">Subject</span>
          {renderEmailTemplate(item.subject, vars)}
        </p>
        <pre className="preview-text">{renderEmailTemplate(item.body_text, vars)}</pre>
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
