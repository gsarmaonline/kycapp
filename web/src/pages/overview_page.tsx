import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { getOrganisation, listMemberships, listRoles, type Organisation } from '../api'
import { OverviewPanel } from '../panels/overview_panel'

export function OverviewPage() {
  const { orgId = '' } = useParams()
  const [org, setOrg] = useState<Organisation | null>(null)
  const [memberCount, setMemberCount] = useState(0)
  const [roleCount, setRoleCount] = useState(0)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      try {
        const [o, m, r] = await Promise.all([
          getOrganisation(orgId),
          listMemberships(orgId),
          listRoles(orgId),
        ])
        setOrg(o)
        setMemberCount(m.items.length)
        setRoleCount(r.items.length)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      }
    })()
  }, [orgId])

  if (error) return <p className="error">{error}</p>
  if (!org) return <p>Loading…</p>
  return <OverviewPanel org={org} memberCount={memberCount} roleCount={roleCount} />
}
