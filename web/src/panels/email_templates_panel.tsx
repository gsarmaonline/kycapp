import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import {
  createEmailTemplate,
  getOrganisation,
  listEmailTemplates,
  updateEmailTemplate,
  type EmailTemplate,
} from '../api'
import { renderEmailTemplate } from '../email_render'

export function EmailTemplatesPanel({
  orgId,
  onError,
}: {
  orgId: string
  onError: (msg: string | null) => void
}) {
  const [templates, setTemplates] = useState<EmailTemplate[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [subject, setSubject] = useState('')
  const [bodyText, setBodyText] = useState('')
  const [bodyHtml, setBodyHtml] = useState('')
  const [name, setName] = useState('')
  const [newKey, setNewKey] = useState('')
  const [newName, setNewName] = useState('')
  const [newSubject, setNewSubject] = useState('')
  const [newBody, setNewBody] = useState('')
  const [sampleDisplayName, setSampleDisplayName] = useState('Pat')
  const [sampleOrgName, setSampleOrgName] = useState('Acme')

  const selected = templates.find((t) => t.id === selectedId) ?? templates[0]

  const sampleVars = useMemo(
    () => ({
      display_name: sampleDisplayName,
      org_name: sampleOrgName,
    }),
    [sampleDisplayName, sampleOrgName],
  )

  const previewSubject = useMemo(
    () => renderEmailTemplate(subject, sampleVars),
    [subject, sampleVars],
  )
  const previewText = useMemo(
    () => renderEmailTemplate(bodyText, sampleVars),
    [bodyText, sampleVars],
  )
  const previewHtml = useMemo(
    () => renderEmailTemplate(bodyHtml, sampleVars),
    [bodyHtml, sampleVars],
  )

  async function refresh() {
    onError(null)
    try {
      const res = await listEmailTemplates(orgId)
      setTemplates(res.items)
      if (!selectedId && res.items[0]) {
        setSelectedId(res.items[0].id)
      }
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Email templates load failed')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  useEffect(() => {
    void getOrganisation(orgId)
      .then((org) => setSampleOrgName(org.name))
      .catch(() => {
        /* keep default */
      })
  }, [orgId])

  useEffect(() => {
    if (!selected) return
    setSelectedId(selected.id)
    setName(selected.name)
    setSubject(selected.subject)
    setBodyText(selected.body_text)
    setBodyHtml(selected.body_html)
  }, [selected?.id, selected?.name, selected?.subject, selected?.body_text, selected?.body_html])

  async function onSave(e: FormEvent) {
    e.preventDefault()
    if (!selected) return
    onError(null)
    try {
      await updateEmailTemplate(selected.id, {
        name,
        subject,
        body_text: bodyText,
        body_html: bodyHtml,
      })
      await refresh()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Save template failed')
    }
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      const created = await createEmailTemplate(orgId, {
        key: newKey,
        name: newName,
        subject: newSubject,
        body_text: newBody,
      })
      setNewKey('')
      setNewName('')
      setNewSubject('')
      setNewBody('')
      await refresh()
      setSelectedId(created.id)
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Create template failed')
    }
  }

  return (
    <section className="email-templates">
      <p className="lede">
        Message copy for app users. Use placeholders like{' '}
        <code>{'{{display_name}}'}</code> and <code>{'{{org_name}}'}</code>. Sending hooks up later
        via workflows.
      </p>

      <form className="create stacked" onSubmit={onCreate}>
        <label>
          New key
          <input value={newKey} onChange={(e) => setNewKey(e.target.value)} placeholder="custom_notice" required />
        </label>
        <label>
          Name
          <input value={newName} onChange={(e) => setNewName(e.target.value)} required />
        </label>
        <label>
          Subject
          <input value={newSubject} onChange={(e) => setNewSubject(e.target.value)} required />
        </label>
        <label>
          Body
          <textarea value={newBody} onChange={(e) => setNewBody(e.target.value)} rows={3} required />
        </label>
        <button type="submit">Create template</button>
      </form>

      <div className="role-toolbar">
        <label>
          Edit template
          <select
            value={selected?.id ?? ''}
            onChange={(e) => setSelectedId(e.target.value)}
          >
            {templates.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} ({t.key}){t.is_system ? ' · system' : ''}
              </option>
            ))}
          </select>
        </label>
      </div>

      {selected && (
        <>
          <form className="create stacked" onSubmit={onSave}>
            <label>
              Name
              <input value={name} onChange={(e) => setName(e.target.value)} required />
            </label>
            <label>
              Key
              <input value={selected.key} disabled />
            </label>
            <label>
              Subject
              <input value={subject} onChange={(e) => setSubject(e.target.value)} required />
            </label>
            <label>
              Body (text)
              <textarea value={bodyText} onChange={(e) => setBodyText(e.target.value)} rows={6} />
            </label>
            <label>
              Body (HTML)
              <textarea value={bodyHtml} onChange={(e) => setBodyHtml(e.target.value)} rows={4} />
            </label>
            <button type="submit">Save</button>
          </form>

          <fieldset className="perm-group email-preview">
            <legend>Preview</legend>
            <div className="create stacked preview-vars">
              <label>
                Sample {'{{display_name}}'}
                <input
                  value={sampleDisplayName}
                  onChange={(e) => setSampleDisplayName(e.target.value)}
                />
              </label>
              <label>
                Sample {'{{org_name}}'}
                <input value={sampleOrgName} onChange={(e) => setSampleOrgName(e.target.value)} />
              </label>
            </div>
            <p className="preview-subject">
              <span className="preview-label">Subject</span>
              {previewSubject || '—'}
            </p>
            <pre className="preview-text">{previewText || '—'}</pre>
            {bodyHtml.trim() ? (
              <iframe
                title="HTML preview"
                className="preview-html"
                sandbox=""
                srcDoc={previewHtml}
              />
            ) : (
              <p className="status">No HTML body — text preview only.</p>
            )}
          </fieldset>
        </>
      )}
      {templates.length === 0 && <p className="empty">No email templates yet.</p>}
    </section>
  )
}
