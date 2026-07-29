import { Link, NavLink, Outlet, useParams } from 'react-router-dom'
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
  const base = docsBasePath(orgId)

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
        Integration APIs for merchant backends, the full operator/platform OpenAPI, and the shared
        variable path vocabulary.
      </p>
      <nav className="docs-tabs" aria-label="Documentation sections">
        <NavLink to={base} end className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}>
          Integration API
        </NavLink>
        <NavLink
          to={`${base}/operator`}
          className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}
        >
          Operator API
        </NavLink>
        <NavLink
          to={`${base}/variables`}
          className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}
        >
          Variables
        </NavLink>
      </nav>
      <Outlet />
    </section>
  )
}
