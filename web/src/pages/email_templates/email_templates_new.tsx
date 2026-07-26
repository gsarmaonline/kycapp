import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { createEmailTemplate } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'

export function EmailTemplatesNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [subject, setSubject] = useState('')
  const [bodyText, setBodyText] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const t = await createEmailTemplate(orgId, {
        key,
        name,
        subject,
        body_text: bodyText,
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
        <label>
          Body
          <span className="field-hint">
            Inner content only — header, logo, footer, and other styling are carried over from{' '}
            <Link to={orgPath(orgId, 'branding')}>Branding</Link>.
          </span>
          <textarea value={bodyText} onChange={(e) => setBodyText(e.target.value)} rows={5} required />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'email-templates')} submitLabel="Create" />
      </form>
    </section>
  )
}
