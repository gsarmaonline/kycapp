import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAutomation, listAutomations, type Automation } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function AutomationsIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<Automation[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAutomations(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load automations')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(a: Automation) {
    if (!confirm(`Delete automation ${a.name || a.trigger}?`)) return
    try {
      await deleteAutomation(a.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Automations"
        createTo={resourcePath(orgId, 'automations', 'new')}
        createLabel="Create automation"
      />
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Name', 'Trigger', 'Enabled', 'Actions']}
          empty="No automations yet."
          rows={items.map((a) => ({
            key: a.id,
            cells: [
              a.name || '—',
              a.trigger,
              a.enabled ? 'yes' : 'no',
              String(a.actions?.length ?? 0),
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'automations', a.id)}
                editTo={resourcePath(orgId, 'automations', a.id, 'edit')}
                onDelete={() => void onDelete(a)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
