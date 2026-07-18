import { Link } from 'react-router-dom'
import type { User } from '../api'
import './landing.css'

export function LandingPage({ user }: { user: User | null }) {
  const primaryTo = user ? '/app' : '/login'
  const primaryLabel = user ? 'Dashboard' : 'Sign in'

  return (
    <div className="landing">
      <header className="landing-top">
        <span className="landing-mark" aria-hidden="true">
          KYC
        </span>
        <Link className="landing-top-link" to={primaryTo}>
          {primaryLabel}
        </Link>
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
            Members, roles, end users, messaging, and billing — one place, not five dashboards.
          </p>
          <div className="landing-cta">
            <Link className="landing-cta-primary" to={primaryTo}>
              {primaryLabel}
            </Link>
            <a className="landing-cta-secondary" href="#what-it-holds">
              See what it holds
            </a>
          </div>
        </div>
      </section>

      <section className="landing-section" id="what-it-holds">
        <h2>Organisation is the hub</h2>
        <p>
          Every membership, permission, app-user profile, and plan hangs off the organisation —
          so tenant state stops living in auth providers, Stripe tabs, and spreadsheets.
        </p>
      </section>

      <section className="landing-section landing-section-alt">
        <h2>Access that matches how you work</h2>
        <p>
          Roles and permissions decide what people may do. Entitlements decide what the
          organisation has paid for. Checks stay clear at runtime.
        </p>
      </section>

      <section className="landing-section">
        <h2>Operate the customer lifecycle</h2>
        <p>
          Schema for end users, branded email, and simple automations — so onboarding and follow-ups
          run from the same record that owns the org.
        </p>
      </section>

      <footer className="landing-footer">
        <span className="landing-mark">KYC</span>
        <Link to={primaryTo}>{user ? 'Go to dashboard' : 'Sign in to continue'}</Link>
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
          <li className="is-active">Overview</li>
          <li>Members</li>
          <li>Users</li>
          <li>Automations</li>
          <li>Billing</li>
        </ul>
      </aside>
      <div className="landing-stage-main">
        <header>
          <span>Overview</span>
          <em>Acme Logistics</em>
        </header>
        <div className="landing-stage-grid">
          <div>
            <b>12</b>
            <span>Members</span>
          </div>
          <div>
            <b>48</b>
            <span>Users</span>
          </div>
          <div>
            <b>6</b>
            <span>Automations</span>
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
