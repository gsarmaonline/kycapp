import { Link } from 'react-router-dom'
import type { OrgOnboarding } from '../api'
import { orgPath, type OrgSection } from '../org_nav'

export function OnboardingChecklist({
  orgId,
  onboarding,
  busy,
  onDismiss,
}: {
  orgId: string
  onboarding: OrgOnboarding
  busy?: boolean
  onDismiss: () => void
}) {
  if (!onboarding.visible) return null

  return (
    <section className="onboarding" aria-labelledby="onboarding-title">
      <div className="onboarding-header">
        <div>
          <h2 id="onboarding-title" className="onboarding-title">
            Getting started
          </h2>
          <p className="lede onboarding-progress">
            {onboarding.completed_count} of {onboarding.total_count} complete — configure your
            product surface, then connect your app.
          </p>
        </div>
        <button type="button" className="ghost" disabled={busy} onClick={onDismiss}>
          Dismiss
        </button>
      </div>
      <ol className="onboarding-steps">
        {onboarding.steps.map((step) => (
          <li key={step.key} className={step.done ? 'done' : 'pending'}>
            <span className="onboarding-marker" aria-hidden="true">
              {step.done ? '✓' : '○'}
            </span>
            {step.done ? (
              <span className="onboarding-label">
                <span className="onboarding-done-label">{step.label}</span>
              </span>
            ) : (
              <Link className="onboarding-label" to={orgPath(orgId, step.href as OrgSection)}>
                {step.label}
              </Link>
            )}
          </li>
        ))}
      </ol>
    </section>
  )
}
