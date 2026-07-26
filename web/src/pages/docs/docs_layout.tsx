import { NavLink, Outlet, useParams } from 'react-router-dom'
import { PageHeader } from '../../crud/ui'
import { orgPath } from '../../org_nav'

export function DocsLayout() {
  const { orgId = '' } = useParams()
  const base = orgPath(orgId, 'docs')

  return (
    <section className="docs-section">
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
