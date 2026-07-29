import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  dismissOrgOnboarding,
  getOrgOnboarding,
  listAppUsers,
  listAttributeDefinitions,
  listAutomations,
  listEmailTemplates,
  listMemberships,
  listOrgActivity,
  listProductFeatures,
  listProductPlans,
  type ActivityEvent,
  type OrgOnboarding,
} from '../api'
import { OverviewPanel } from '../panels/overview_panel'

export function OverviewPage() {
  const { orgId = '' } = useParams()
  const [ready, setReady] = useState(false)
  const [memberCount, setMemberCount] = useState(0)
  const [userCount, setUserCount] = useState(0)
  const [attributeCount, setAttributeCount] = useState(0)
  const [templateCount, setTemplateCount] = useState(0)
  const [automationCount, setAutomationCount] = useState(0)
  const [featureCount, setFeatureCount] = useState(0)
  const [productPlanCount, setProductPlanCount] = useState(0)
  const [onboarding, setOnboarding] = useState<OrgOnboarding | null>(null)
  const [recentActivity, setRecentActivity] = useState<ActivityEvent[] | undefined>(undefined)
  const [activityError, setActivityError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      setReady(false)
      setError(null)
      setActivityError(null)
      setRecentActivity(undefined)
      try {
        const [m, users, attrs, templates, automations, features, productPlans, onboard] =
          await Promise.all([
            listMemberships(orgId),
            listAppUsers(orgId),
            listAttributeDefinitions(orgId),
            listEmailTemplates(orgId),
            listAutomations(orgId),
            listProductFeatures(orgId),
            listProductPlans(orgId),
            getOrgOnboarding(orgId),
          ])
        setMemberCount(m.items.length)
        setUserCount(users.items.length)
        setAttributeCount(attrs.items.length)
        setTemplateCount(templates.items.length)
        setAutomationCount(automations.items.length)
        setFeatureCount(features.items.length)
        setProductPlanCount(productPlans.items.length)
        setOnboarding(onboard)
        setReady(true)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      }

      try {
        const act = await listOrgActivity(orgId, 8)
        setRecentActivity(act.items)
      } catch (e) {
        setRecentActivity([])
        setActivityError(e instanceof Error ? e.message : 'Failed to load activity')
      }
    })()
  }, [orgId])

  async function onDismiss() {
    setBusy(true)
    setError(null)
    try {
      setOnboarding(await dismissOrgOnboarding(orgId))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Dismiss failed')
    } finally {
      setBusy(false)
    }
  }

  if (error && !ready) return <p className="error">{error}</p>
  if (!ready) return <p>Loading…</p>

  return (
    <>
      {error && <p className="error">{error}</p>}
      <OverviewPanel
        orgId={orgId}
        onboarding={onboarding}
        onboardingBusy={busy}
        onDismissOnboarding={() => void onDismiss()}
        recentActivity={recentActivity}
        activityError={activityError}
        tiles={[
          { label: 'Members', value: memberCount, to: 'members' },
          { label: 'Users', value: userCount, to: 'users' },
          { label: 'User Attributes', value: attributeCount, to: 'attributes' },
          { label: 'Emails', value: templateCount, to: 'email-templates' },
          { label: 'Automations', value: automationCount, to: 'automations' },
          { label: 'Features', value: featureCount, to: 'product-features' },
          { label: 'Plans', value: productPlanCount, to: 'product-plans' },
        ]}
      />
    </>
  )
}
