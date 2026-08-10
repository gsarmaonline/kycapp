import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAppCapability, listAppCapabilities, type AppCapability } from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function CustomerCapabilitiesIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AppCapability[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAppCapabilities(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load capabilities')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(c: AppCapability) {
    if (!confirm(`Delete capability ${c.key}?`)) return
    try {
      await deleteAppCapability(orgId, c.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Capabilities"
        createTo={resourcePath(orgId, 'customer-capabilities', 'new')}
        createLabel="Add capability"
      />
      <p className="muted">
        The verbs your product checks, as <code>resource:action</code>. A role can only use
        capabilities declared here, so a typo is caught rather than silently granting nothing.
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Key', 'Description']}
          empty="No capabilities yet."
          rows={items.map((c) => ({
            key: c.id,
            cells: [c.key, c.description || '—'],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'customer-capabilities', c.id)}
                editTo={resourcePath(orgId, 'customer-capabilities', c.id, 'edit')}
                onDelete={() => void onDelete(c)}
              />
            ),
          }))}
        />
      )}
    </section>
  )
}
