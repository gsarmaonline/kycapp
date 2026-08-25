import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getRole, type Role } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function RolesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<Role | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getRole(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Role" />
      <DetailList
        items={[
          { label: 'Name', value: item.name },
          { label: 'Key', value: item.key },
          { label: 'System', value: item.is_system ? 'yes' : 'no' },
          { label: 'Description', value: item.description || '—' },
          {
            label: 'Permissions',
            value: item.permission_keys?.length ? item.permission_keys.join(', ') : '—',
          },
        ]}
        editTo={resourcePath(orgId, 'roles', item.id, 'edit')}
        backTo={resourcePath(orgId, 'roles')}
      />
    </section>
  )
}
