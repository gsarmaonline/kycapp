import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getAppCapability, type AppCapability } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'

export function CustomerCapabilitiesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<AppCapability | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAppCapability(orgId, id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Capability" />
      <DetailList
        items={[
          { label: 'Key', value: item.key },
          { label: 'Description', value: item.description || '—' },
        ]}
        editTo={resourcePath(orgId, 'customer-capabilities', item.id, 'edit')}
        backTo={orgPath(orgId, 'customer-model')}
      />
    </section>
  )
}
