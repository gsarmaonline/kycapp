import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getAppRole, listAppRoles, type AppRole } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'

function CapabilityList({ items }: { items: { key: string; inherited: boolean }[] }) {
  if (items.length === 0) return <span className="cap-empty">none</span>
  return (
    <ul className="cap-list">
      {items.map((c) => (
        <li
          key={c.key}
          className={c.inherited ? 'cap-chip cap-chip-inherited' : 'cap-chip'}
          title={c.inherited ? 'Inherited from a role this one builds on' : 'Granted by this role'}
        >
          {c.key}
          {c.inherited ? <em>inherited</em> : null}
        </li>
      ))}
    </ul>
  )
}

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

  // Marked and grouped so inherited entries are obvious: showing one flat set is
  // how inheritance surprises people.
  const effective = item.effective_capabilities
    .map((key) => ({ key, inherited: !own.has(key) }))
    .sort((a, b) => Number(a.inherited) - Number(b.inherited) || a.key.localeCompare(b.key))

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
            value: (
              <CapabilityList
                items={item.own_capabilities.map((key) => ({ key, inherited: false }))}
              />
            ),
          },
          {
            label: 'Effective capabilities',
            value: <CapabilityList items={effective} />,
          },
        ]}
        editTo={resourcePath(orgId, 'customer-roles', item.id, 'edit')}
        backTo={orgPath(orgId, 'customer-roles-groups')}
      />
    </section>
  )
}
