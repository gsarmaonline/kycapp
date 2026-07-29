import { Link, NavLink, Outlet, useLocation, useParams } from 'react-router-dom'
import type { User } from '../../api'
import { PageHeader } from '../../crud/ui'
import { docsBasePath } from '../../templates/paths'

export function DocsLayout({
  publicChrome = false,
  user = null,
}: {
  publicChrome?: boolean
  user?: User | null
}) {
  const { orgId } = useParams()
  const location = useLocation()
  const base = docsBasePath(orgId)
  const onApi = /\/docs\/api(\/|$)/.test(location.pathname)
  const onConcepts =
    location.pathname === base ||
    location.pathname.endsWith('/docs') ||
    /\/docs\/concepts(\/|$)/.test(location.pathname)

  return (
    <section className="docs-section">
      {publicChrome && (
        <header className="public-docs-top">
          <Link to="/" className="public-docs-brand">
            KYC
          </Link>
          <nav className="public-docs-top-nav" aria-label="Site">
            <Link to="/docs">Docs</Link>
            <Link to={user ? '/app' : '/login'}>{user ? 'Dashboard' : 'Sign in'}</Link>
          </nav>
        </header>
      )}
      <PageHeader title="Documentation" />
      <p className="lede">
        Concepts for workspace elements, the Integration and Operator API references, and the shared
        variable path vocabulary.
      </p>
      <nav className="docs-tabs" aria-label="Documentation sections">
        <NavLink
          to={base}
          end={!onConcepts}
          className={() => (onConcepts ? 'docs-tab active' : 'docs-tab')}
        >
          Concepts
        </NavLink>
        <NavLink
          to={`${base}/api`}
          className={() => (onApi ? 'docs-tab active' : 'docs-tab')}
        >
          API reference
        </NavLink>
        <NavLink
          to={`${base}/variables`}
          className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}
        >
          Variables
        </NavLink>
      </nav>
      {onApi && (
        <nav className="docs-subtabs" aria-label="API reference">
          <NavLink
            to={`${base}/api`}
            end
            className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}
          >
            Integration API
          </NavLink>
          <NavLink
            to={`${base}/api/operator`}
            className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}
          >
            Operator API
          </NavLink>
        </nav>
      )}
      <Outlet />
    </section>
  )
}
