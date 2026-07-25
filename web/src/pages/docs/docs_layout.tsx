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
        API reference (OpenAPI) and the shared variable path vocabulary used in emails, webhooks,
        automations, and database mappings.
      </p>
      <nav className="docs-tabs" aria-label="Documentation sections">
        <NavLink to={base} end className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}>
          API reference
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
