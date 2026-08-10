import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getAppScopeType, type AppScopeType } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'

export function CustomerScopeKindsShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<AppScopeType | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAppScopeType(orgId, id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Scope kind" />
      <DetailList
        items={[
          { label: 'Kind', value: item.kind },
          { label: 'Label', value: item.label || '—' },
        ]}
      />
    </section>
  )
}
