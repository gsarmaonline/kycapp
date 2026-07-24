import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getOrgDatabase, type OrgDatabase } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function DatabasesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<OrgDatabase | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getOrgDatabase(orgId, id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [orgId, id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title={item.name || 'Database'} />
      <DetailList
        items={[
          { label: 'Name', value: item.name },
          { label: 'Driver', value: item.driver },
          { label: 'Host', value: item.host },
          { label: 'Port', value: String(item.port) },
          { label: 'Database', value: item.database_name },
          { label: 'Username', value: item.username },
          { label: 'Password', value: item.has_password ? item.password_hint || '••••' : '—' },
          { label: 'SSL mode', value: item.ssl_mode },
          { label: 'Status', value: item.status },
        ]}
      />
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'databases')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'databases', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
