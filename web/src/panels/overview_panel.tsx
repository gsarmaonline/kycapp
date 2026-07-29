import { Link } from 'react-router-dom'
import type { ActivityEvent, OrgOnboarding } from '../api'
import { OnboardingGuide } from '../components/OnboardingGuide'
import { ActivityFeed } from '../components/ActivityFeed'
import { EntitlementUsageChart } from '../components/EntitlementUsageChart'
import { orgPath, type OrgSection } from '../org_nav'

type Tile = {
  label: string
  value: string | number
  to: OrgSection
}

export function OverviewPanel({
  orgId,
  tiles,
  onboarding,
  onboardingBusy,
  onDismissOnboarding,
  recentActivity,
  activityError,
}: {
  orgId: string
  tiles: Tile[]
  onboarding?: OrgOnboarding | null
  onboardingBusy?: boolean
  onDismissOnboarding?: () => void
  recentActivity?: ActivityEvent[]
  activityError?: string | null
}) {
  return (
    <section className="overview">
      {onboarding && onDismissOnboarding && (
        <OnboardingGuide
          orgId={orgId}
          onboarding={onboarding}
          busy={onboardingBusy}
          onDismiss={onDismissOnboarding}
        />
      )}
      <p className="lede">
        Jump into a section below, or use the sidebar. Switch organisations with the dropdown above.
      </p>
      <ul className="overview-stats">
        {tiles.map((tile) => (
          <li key={tile.to}>
            <Link className="overview-tile" to={orgPath(orgId, tile.to)}>
              <strong>{tile.value}</strong>
              <span>{tile.label}</span>
            </Link>
          </li>
        ))}
      </ul>

      <div className="overview-obs">
        <EntitlementUsageChart orgId={orgId} days={14} title="Entitlement checks" />
        <section className="obs-card">
          <header className="obs-card-header">
            <h3 className="obs-card-title">Recent activity</h3>
            <Link className="obs-card-link" to={orgPath(orgId, 'activity')}>
              View all
            </Link>
          </header>
          {activityError ? <p className="error">{activityError}</p> : null}
          <ActivityFeed
            items={recentActivity ?? []}
            emptyLabel={recentActivity == null ? 'Loading…' : 'No activity yet'}
          />
        </section>
      </div>
    </section>
  )
}
