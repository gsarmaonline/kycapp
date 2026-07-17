import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getAppUser, type AppUser } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function UsersShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<AppUser | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAppUser(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="User" />
      <DetailList
        items={[
          { label: 'Display name', value: item.display_name || '—' },
          { label: 'Email', value: item.email || '—' },
          { label: 'External ID', value: item.external_id || '—' },
          { label: 'Status', value: item.status },
          {
            label: 'Attributes',
            value:
              Object.keys(item.attributes || {}).length === 0
                ? '—'
                : Object.entries(item.attributes)
                    .map(([k, v]) => `${k}=${String(v)}`)
                    .join(', '),
          },
        ]}
      />
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'users')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'users', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
