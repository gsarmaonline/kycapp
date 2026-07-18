import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteAttributeDefinition,
  listAttributeDefinitions,
  type AttributeDefinition,
} from '../../api'
import { PageHeader, ResourceTable, RowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function AttributesIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AttributeDefinition[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAttributeDefinitions(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load attributes')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onDelete(d: AttributeDefinition) {
    if (!confirm(`Archive attribute ${d.label}?`)) return
    try {
      await deleteAttributeDefinition(d.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="User Attributes"
        createTo={resourcePath(orgId, 'attributes', 'new')}
        createLabel="Create attribute"
      />
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Label', 'Key', 'Type', 'Section', 'System', 'Required', 'Status']}
          empty="No attributes defined yet."
          rows={items.map((d) => ({
            key: d.id,
            cells: [
              d.label,
              d.key,
              d.value_type,
              d.section,
              d.is_system ? 'yes' : 'no',
              d.required ? 'yes' : 'no',
              d.status,
            ],
            actions: (
              <RowActions
                viewTo={resourcePath(orgId, 'attributes', d.id)}
                editTo={resourcePath(orgId, 'attributes', d.id, 'edit')}
                onDelete={() => void onDelete(d)}
                deleteDisabled={d.is_system || d.status === 'archived'}
                deleteTitle={
                  d.is_system
                    ? 'System attributes cannot be deleted'
                    : d.status === 'archived'
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
