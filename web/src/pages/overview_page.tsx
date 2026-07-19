import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  listAppUsers,
  listAttributeDefinitions,
  listAutomations,
  listEmailTemplates,
  listMemberships,
  listProductFeatures,
  listProductPlans,
  listRoles,
} from '../api'
import { OverviewPanel } from '../panels/overview_panel'

export function OverviewPage() {
  const { orgId = '' } = useParams()
  const [ready, setReady] = useState(false)
  const [memberCount, setMemberCount] = useState(0)
  const [roleCount, setRoleCount] = useState(0)
  const [userCount, setUserCount] = useState(0)
  const [attributeCount, setAttributeCount] = useState(0)
  const [templateCount, setTemplateCount] = useState(0)
  const [automationCount, setAutomationCount] = useState(0)
  const [featureCount, setFeatureCount] = useState(0)
  const [productPlanCount, setProductPlanCount] = useState(0)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      try {
        const [m, r, users, attrs, templates, automations, features, productPlans] =
          await Promise.all([
            listMemberships(orgId),
            listRoles(orgId),
            listAppUsers(orgId),
            listAttributeDefinitions(orgId),
            listEmailTemplates(orgId),
            listAutomations(orgId),
            listProductFeatures(orgId),
            listProductPlans(orgId),
          ])
        setMemberCount(m.items.length)
        setRoleCount(r.items.length)
        setUserCount(users.items.length)
        setAttributeCount(attrs.items.length)
        setTemplateCount(templates.items.length)
        setAutomationCount(automations.items.length)
        setFeatureCount(features.items.length)
        setProductPlanCount(productPlans.items.length)
        setReady(true)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      }
    })()
  }, [orgId])

  if (error) return <p className="error">{error}</p>
  if (!ready) return <p>Loading…</p>

  return (
    <OverviewPanel
      orgId={orgId}
      tiles={[
        { label: 'Members', value: memberCount, to: 'members' },
        { label: 'Roles', value: roleCount, to: 'roles' },
        { label: 'Users', value: userCount, to: 'users' },
        { label: 'User Attributes', value: attributeCount, to: 'attributes' },
        { label: 'Email templates', value: templateCount, to: 'email-templates' },
        { label: 'Automations', value: automationCount, to: 'automations' },
        { label: 'Features', value: featureCount, to: 'product-features' },
        { label: 'Plans', value: productPlanCount, to: 'product-plans' },
      ]}
    />
  )
}
