import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getAppRole, listAppRoles, type AppRole } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'

export function CustomerRolesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<(AppRole & { extends: string[] }) | null>(null)
  const [all, setAll] = useState<AppRole[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([getAppRole(orgId, id), listAppRoles(orgId)])
      .then(([r, list]) => {
        setItem(r)
        setAll(list.items)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  const own = new Set(item.own_capabilities)
  const parentNames = item.extends
    .map((pid) => all.find((r) => r.id === pid)?.name ?? pid)
    .join(', ')

  return (
    <section>
      <PageHeader title="Role" />
      <DetailList
        items={[
          { label: 'Name', value: item.name },
          { label: 'Key', value: item.key },
          { label: 'Description', value: item.description || '—' },
          { label: 'Builds on', value: parentNames || 'nothing' },
          {
            label: 'Own capabilities',
            value: item.own_capabilities.length ? item.own_capabilities.join(', ') : 'none',
          },
          {
            // Marked so inherited entries are obvious: showing only the chain
            // is how inheritance surprises people.
            label: 'Effective capabilities',
            value: item.effective_capabilities.length
              ? item.effective_capabilities.map((c) => (own.has(c) ? c : `${c} (inherited)`)).join(', ')
              : 'none',
          },
        ]}
      />
    </section>
  )
}
