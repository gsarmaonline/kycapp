import { Link } from 'react-router-dom'
import type { User } from '../api'
import './landing.css'

const FEATURES = [
  {
    id: 'organisations',
    title: 'Organisations, members, and roles',
    body: 'Tenants are first-class. Invite operators, assign permissions, and keep every membership scoped to the organisation.',
  },
  {
    id: 'customers',
    title: 'App users and profile schema',
    body: 'Store end-customer profiles with typed attributes, or ingest from your auth provider by external id — KYC stays the system of record.',
  },
  {
    id: 'packaging',
    title: 'Features, plans, and entitlement checks',
    body: 'Package product features into plans, then gate access from your backend with a single entitlements check.',
  },
  {
    id: 'lifecycle',
    title: 'Email, branding, and automations',
    body: 'Branded templates and rule-based automations run off the same customer events — welcome mail, follow-ups, and lifecycle hooks.',
  },
  {
    id: 'connect',
    title: 'API keys, webhooks, and databases',
    body: 'Connect your app with org API keys, fire outbound webhooks, accept inbound triggers, and write to connected databases from automations.',
  },
  {
    id: 'billing',
    title: 'Billing that owns access state',
    body: 'Stripe runs Checkout and Portal; KYC owns subscription and entitlement state so access stays consistent with the plan.',
  },
] as const

export function LandingPage({ user }: { user: User | null }) {
  const primaryTo = user ? '/app' : '/login'
  const primaryLabel = user ? 'Dashboard' : 'Sign in'

  return (
    <div className="landing">
      <header className="landing-top">
        <span className="landing-mark" aria-hidden="true">
          KYC
        </span>
        <nav className="landing-top-nav" aria-label="Site">
          <Link className="landing-top-link" to="/docs">
            Docs
          </Link>
          <Link className="landing-top-link" to={primaryTo}>
            {primaryLabel}
          </Link>
        </nav>
      </header>

      <section className="landing-hero" aria-label="KYC">
        <div className="landing-hero-visual" aria-hidden="true">
          <div className="landing-hero-plane">
            <ProductStage />
          </div>
        </div>

        <div className="landing-hero-copy">
          <p className="landing-brand">KYC</p>
          <h1 className="landing-headline">The system of record for organisations.</h1>
          <p className="landing-support">
            Configure orgs, customers, packaging, and lifecycle in one place — enforce in your
            backend.
          </p>
          <div className="landing-cta">
            <Link className="landing-cta-primary" to={primaryTo}>
              {primaryLabel}
            </Link>
            <a className="landing-cta-secondary" href="#features">
              See features
            </a>
          </div>
        </div>
      </section>

      <section className="landing-features" id="features" aria-label="Features">
        {FEATURES.map((feature, index) => (
          <section
            key={feature.id}
            className={
              index % 2 === 1 ? 'landing-section landing-section-alt' : 'landing-section'
            }
            id={feature.id}
          >
            <h2>{feature.title}</h2>
            <p>{feature.body}</p>
          </section>
        ))}
      </section>

      <section className="landing-section landing-section-close">
        <h2>Configure here. Enforce in your API.</h2>
        <p>
          Merchants keep login where it belongs. KYC stores the record, runs the automations, and
          answers entitlement checks — start with the{' '}
          <Link className="landing-inline-link" to="/docs">
            docs
          </Link>{' '}
          (concepts and the{' '}
          <Link className="landing-inline-link" to="/docs/api">
            Integration API
          </Link>
          ).
        </p>
      </section>

      <footer className="landing-footer">
        <span className="landing-mark">KYC</span>
        <nav className="landing-footer-nav" aria-label="Footer">
          <Link to="/docs">Docs</Link>
          <Link to={primaryTo}>{user ? 'Go to dashboard' : 'Sign in to continue'}</Link>
        </nav>
      </footer>
    </div>
  )
}

function ProductStage() {
  return (
    <div className="landing-stage">
      <aside className="landing-stage-nav">
        <strong>KYC</strong>
        <span>Acme Logistics</span>
        <ul>
          <li className="is-active">Users</li>
          <li>Features</li>
          <li>Automations</li>
          <li>Billing</li>
          <li>API keys</li>
        </ul>
      </aside>
      <div className="landing-stage-main">
        <header>
          <span>Users</span>
          <em>Acme Logistics</em>
        </header>
        <div className="landing-stage-grid">
          <div>
            <b>48</b>
            <span>App users</span>
          </div>
          <div>
            <b>6</b>
            <span>Features</span>
          </div>
          <div>
            <b>3</b>
            <span>Plans</span>
          </div>
        </div>
        <div className="landing-stage-flow">
          <i />
          <i />
          <i />
        </div>
      </div>
    </div>
  )
}
