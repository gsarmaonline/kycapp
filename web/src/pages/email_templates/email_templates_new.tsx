import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { createEmailTemplate } from '../../api'
import { EmailBodySectionsEditor } from '../../components/EmailBodySectionsEditor'
import { FormActions, PageHeader } from '../../crud/ui'
import { newBodySection, type EmailBodySection } from '../../email_fonts'
import { orgPath, resourcePath } from '../../org_nav'

export function EmailTemplatesNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [subject, setSubject] = useState('')
  const [bodyText, setBodyText] = useState('')
  const [sections, setSections] = useState<EmailBodySection[]>([
    newBodySection('<p>Hi {{app_user.display_name}},</p>'),
  ])
  const [fromName, setFromName] = useState('')
  const [fromAddress, setFromAddress] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const t = await createEmailTemplate(orgId, {
        key,
        name,
        subject,
        body_text: bodyText || sections.map((s) => s.content_html).join('\n'),
        body_sections: sections,
        from_name: fromName,
        from_address: fromAddress,
      })
      navigate(resourcePath(orgId, 'email-templates', t.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Create email template" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input value={key} onChange={(e) => setKey(e.target.value)} required />
        </label>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Subject
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
            <input value={fromName} onChange={(e) => setFromName(e.target.value)} />
          </label>
          <label>
            From address
            <input value={fromAddress} onChange={(e) => setFromAddress(e.target.value)} />
          </label>
        </fieldset>
        <label>
          Body (text)
          <textarea value={bodyText} onChange={(e) => setBodyText(e.target.value)} rows={3} />
        </label>
        <EmailBodySectionsEditor sections={sections} onChange={setSections} brandingHint />
        <FormActions cancelTo={resourcePath(orgId, 'email-templates')} submitLabel="Create" />
      </form>
    </section>
  )
}
