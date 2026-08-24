import { useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useParams } from 'react-router-dom'
import type { User } from '../../api'
import { PageHeader } from '../../crud/ui'
import { docsBasePath } from '../../templates/paths'
import { searchDocs } from './docs_search'

export function DocsLayout({
  publicChrome = false,
  user = null,
}: {
  publicChrome?: boolean
  user?: User | null
}) {
  const { orgId } = useParams()
  const location = useLocation()
  const [query, setQuery] = useState('')
  const base = docsBasePath(orgId)
  const hits = searchDocs(query)
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
        <NavLink
          to={`${base}/authorisation`}
          className={({ isActive }) => (isActive ? 'docs-tab active' : 'docs-tab')}
        >
          Authorisation
        </NavLink>
      </nav>
      <form className="docs-search" role="search" onSubmit={(e) => e.preventDefault()}>
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search the docs"
          aria-label="Search the docs"
        />
        {query && (
          <button type="button" className="link-btn" onClick={() => setQuery('')}>
            Clear
          </button>
        )}
      </form>

      {onApi && !query && (
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
      {/*
        While searching, results replace the page rather than sitting beside it.
        Two competing lists on one screen is what makes in-app docs search feel
        worse than no search at all.
      */}
      {query ? (
        <article className="docs-concepts prose-docs">
          <p className="docs-search-count">
            {hits.length === 0
              ? `Nothing matches \u201c${query}\u201d.`
              : `${hits.length} ${hits.length === 1 ? 'result' : 'results'} for \u201c${query}\u201d`}
          </p>
          <ul className="docs-concept-list">
            {hits.map((hit) => (
              <li key={hit.slug}>
                <Link to={`${base}/concepts/${hit.slug}`} onClick={() => setQuery('')}>
                  <strong>{hit.title}</strong>
                  <span>{hit.excerpt}</span>
                </Link>
              </li>
            ))}
          </ul>
        </article>
      ) : (
        <Outlet />
      )}
    </section>
  )
}
