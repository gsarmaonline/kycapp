import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getMembership, type Membership } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'
import { AccessPathPanel } from '../../panels/access_path_panel'

export function MembersShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<Membership | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getMembership(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Member" />
      <DetailList
        items={[
          { label: 'Name', value: item.user_name || '—' },
          { label: 'Email', value: item.user_email || '—' },
          { label: 'Role', value: item.role_key || '—' },
          { label: 'Status', value: item.status },
          { label: 'User ID', value: item.user_id },
          { label: 'Membership ID', value: item.id },
        ]}
      />
      <AccessPathPanel membershipId={item.id} />
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'members')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'members', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
