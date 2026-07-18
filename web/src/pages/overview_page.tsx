import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  getOrganisation,
  listAppUsers,
  listAttributeDefinitions,
  listAutomations,
  listEmailTemplates,
  listMemberships,
  listRoles,
  type Organisation,
} from '../api'
import { OverviewPanel } from '../panels/overview_panel'

export function OverviewPage() {
  const { orgId = '' } = useParams()
  const [org, setOrg] = useState<Organisation | null>(null)
  const [memberCount, setMemberCount] = useState(0)
  const [roleCount, setRoleCount] = useState(0)
  const [userCount, setUserCount] = useState(0)
  const [attributeCount, setAttributeCount] = useState(0)
  const [templateCount, setTemplateCount] = useState(0)
  const [automationCount, setAutomationCount] = useState(0)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      try {
        const [o, m, r, users, attrs, templates, automations] = await Promise.all([
          getOrganisation(orgId),
          listMemberships(orgId),
          listRoles(orgId),
          listAppUsers(orgId),
          listAttributeDefinitions(orgId),
          listEmailTemplates(orgId),
          listAutomations(orgId),
        ])
        setOrg(o)
        setMemberCount(m.items.length)
        setRoleCount(r.items.length)
        setUserCount(users.items.length)
        setAttributeCount(attrs.items.length)
        setTemplateCount(templates.items.length)
        setAutomationCount(automations.items.length)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      }
    })()
  }, [orgId])

  if (error) return <p className="error">{error}</p>
  if (!org) return <p>Loading…</p>

  return (
    <OverviewPanel
      orgId={orgId}
      org={org}
      tiles={[
        { label: 'Members', value: memberCount, to: 'members' },
        { label: 'Roles', value: roleCount, to: 'roles' },
        { label: 'Users', value: userCount, to: 'users' },
        { label: 'User Attributes', value: attributeCount, to: 'attributes' },
        { label: 'Email templates', value: templateCount, to: 'email-templates' },
        { label: 'Automations', value: automationCount, to: 'automations' },
        { label: 'Branding', value: 'Open', to: 'branding' },
        { label: 'Billing', value: 'Open', to: 'billing' },
      ]}
    />
  )
}
