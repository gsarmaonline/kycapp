import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteEmailTemplate, listEmailTemplates, type EmailTemplate } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function EmailTemplatesIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<EmailTemplate[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listEmailTemplates(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load templates')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(t: EmailTemplate) {
    if (!confirm(`Delete template ${t.name}?`)) return
    try {
      await deleteEmailTemplate(t.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Email templates"
        createTo={resourcePath(orgId, 'email-templates', 'new')}
        createLabel="Create email template"
      />
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Key', 'Subject', 'System', 'Status']}
          empty="No email templates yet."
          rows={items.map((t) => ({
            key: t.id,
            cells: [t.name, t.key, t.subject, t.is_system ? 'yes' : 'no', t.status],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'email-templates', t.id)}
                editTo={resourcePath(orgId, 'email-templates', t.id, 'edit')}
                onDelete={() => void onDelete(t)}
                deleteDisabled={t.is_system || t.status === 'archived'}
                deleteTitle={
                  t.is_system
                    ? 'System templates cannot be deleted'
                    : t.status === 'archived'
                      ? 'Already archived'
                      : undefined
                }
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
