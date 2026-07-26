import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { OnboardingStep, OrgOnboarding } from '../api'
import { orgPath, type OrgSection } from '../org_nav'

type StepGuide = {
  why: string
  how: string
  cta: string
}

const STEP_GUIDE: Record<string, StepGuide> = {
  branding: {
    why: 'Outbound mail should read as your product, not a generic system notice.',
    how: 'On Branding, add a logo, set a primary colour other than the default green, or write an email footer.',
    cta: 'Open branding',
  },
  features: {
    why: 'Features are the capabilities you grant through plans — without them, plans have nothing to unlock.',
    how: 'Create a product feature with a stable key your app can check (for example `billing` or `sso`).',
    cta: 'Define a feature',
  },
  plan: {
    why: 'Plans package features into what you sell or assign to customers.',
    how: 'Create a product plan and attach the features that belong on it.',
    cta: 'Package a plan',
  },
  automation: {
    why: 'Automations run follow-ups when users join, change plan, or hit other lifecycle events.',
    how: 'Create an automation, pick a trigger, and add at least one action such as send email.',
    cta: 'Add an automation',
  },
  api_key: {
    why: 'Your app needs an API key to create users and read entitlements from KYC.',
    how: 'Create an active API key in API keys, then store the secret in your app config.',
    cta: 'Create an API key',
  },
  app_user: {
    why: 'An app user is the first end-user record — proof the integration path works end to end.',
    how: 'Create a user manually in Users, or call the users API with the key you just made.',
    cta: 'Create an app user',
  },
}

function guideFor(step: OnboardingStep): StepGuide {
  return (
    STEP_GUIDE[step.key] ?? {
      why: 'This step finishes org setup so the rest of the product can rely on it.',
      how: `Open ${step.label} and complete the form there.`,
      cta: step.label,
    }
  )
}

export function OnboardingGuide({
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
  if (onboarding.steps.length === 0) return null
  if (onboarding.completed_count >= onboarding.total_count) return null

  const pending = onboarding.steps.filter((s) => !s.done)
  const done = onboarding.steps.filter((s) => s.done)
  const firstPendingKey = pending[0]?.key ?? null
  const next = pending[0]

  if (onboarding.dismissed) {
    if (!next) return null
    return (
      <section className="onboarding onboarding--compact" aria-labelledby="onboarding-title">
        <div className="onboarding-compact-row">
          <div>
            <h2 id="onboarding-title" className="onboarding-title">
              Getting started
            </h2>
            <p className="onboarding-compact-meta">
              {onboarding.completed_count} of {onboarding.total_count} ready · Next: {next.label}
            </p>
          </div>
          <Link className="onboarding-cta" to={orgPath(orgId, next.href as OrgSection)}>
            {guideFor(next).cta}
          </Link>
        </div>
      </section>
    )
  }

  if (!onboarding.visible) return null

  return (
    <OnboardingGuideBody
      key={firstPendingKey ?? 'done'}
      orgId={orgId}
      pending={pending}
      done={done}
      firstPendingKey={firstPendingKey}
      busy={busy}
      onDismiss={onDismiss}
    />
  )
}

function OnboardingGuideBody({
  orgId,
  pending,
  done,
  firstPendingKey,
  busy,
  onDismiss,
}: {
  orgId: string
  pending: OnboardingStep[]
  done: OnboardingStep[]
  firstPendingKey: string | null
  busy?: boolean
  onDismiss: () => void
}) {
  const [focusKey, setFocusKey] = useState(firstPendingKey)
  const focus = pending.find((s) => s.key === focusKey) ?? pending[0]
  if (!focus) return null

  const guide = guideFor(focus)
  const ahead = pending.filter((s) => s.key !== focus.key)

  return (
    <section className="onboarding" aria-labelledby="onboarding-title">
      <div className="onboarding-header">
        <div>
          <h2 id="onboarding-title" className="onboarding-title">
            Getting started
          </h2>
          <p className="lede onboarding-intro">
            A short path to a working product surface — each step unlocks the next.
          </p>
        </div>
        <button type="button" className="ghost" disabled={busy} onClick={onDismiss}>
          Dismiss
        </button>
      </div>

      <div className="onboarding-focus">
        <p className="onboarding-eyebrow">
          {focus.key === firstPendingKey ? 'Suggested next' : 'Up next'}
        </p>
        <h3 className="onboarding-focus-title">{focus.label}</h3>
        <dl className="onboarding-explain">
          <div>
            <dt>Why</dt>
            <dd>{guide.why}</dd>
          </div>
          <div>
            <dt>How</dt>
            <dd>{guide.how}</dd>
          </div>
        </dl>
        <Link className="onboarding-cta" to={orgPath(orgId, focus.href as OrgSection)}>
          {guide.cta}
        </Link>
      </div>

      {(ahead.length > 0 || done.length > 0) && (
        <div className="onboarding-path">
          {ahead.length > 0 && (
            <div className="onboarding-path-block">
              <p className="onboarding-path-label">Also ahead</p>
              <ul className="onboarding-ahead">
                {ahead.map((step) => (
                  <li key={step.key}>
                    <button
                      type="button"
                      className="onboarding-ahead-btn"
                      onClick={() => setFocusKey(step.key)}
                    >
                      {step.label}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {done.length > 0 && (
            <p className="onboarding-ready">
              Ready: {done.map((s) => s.label).join(' · ')}
            </p>
          )}
        </div>
      )}
    </section>
  )
}
