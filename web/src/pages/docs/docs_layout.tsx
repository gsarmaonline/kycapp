import { useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useParams } from 'react-router-dom'
import type { User } from '../../api'
import { PageHeader } from '../../crud/ui'
import { docsBasePath } from '../../templates/paths'
import { conceptNavGroups } from './docs_concepts'
import { searchDocs } from './docs_search'

/**
 * Documentation, navigated from a sidebar.
 *
 * It used to be a row of tabs. Tabs work while a set is small and flat, and
 * this one is neither: three destinations, one of which fans out into two API
 * references, and twenty concepts that were reachable only by first landing on
 * an index and scrolling. The nesting a docs set needs is exactly what a tab
 * row cannot express, so every concept sat one page further away than it
 * should.
 *
 * The sidebar carries the whole tree at once, so the current page is visible in
 * the context of its siblings, which is the thing tabs were hiding.
 */
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

      <div className="docs-shell">
        <aside className="docs-aside">
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

          <nav className="docs-nav" aria-label="Documentation sections">
            <ConceptsNav base={base} pathname={location.pathname} onNavigate={() => setQuery('')} />

            <p className="docs-nav-label">API reference</p>
            <NavLink end to={`${base}/api`} className={navClass} onClick={() => setQuery('')}>
              Integration API
            </NavLink>
            <NavLink to={`${base}/api/operator`} className={navClass} onClick={() => setQuery('')}>
              Operator API
            </NavLink>

            <p className="docs-nav-label">Reference</p>
            <NavLink to={`${base}/variables`} className={navClass} onClick={() => setQuery('')}>
              Variables
            </NavLink>
          </nav>
        </aside>

        {/*
          While searching, results replace the page rather than sitting beside
          it. Two competing lists on one screen is what makes in-app docs search
          feel worse than no search at all.
        */}
        <div className="docs-body">
          {query ? (
            <article className="docs-concepts prose-docs">
              <p className="docs-search-count">
                {hits.length === 0
                  ? `Nothing matches “${query}”.`
                  : `${hits.length} ${hits.length === 1 ? 'result' : 'results'} for “${query}”`}
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
        </div>
      </div>
    </section>
  )
}

function navClass({ isActive }: { isActive: boolean }) {
  return isActive ? 'docs-nav-item active' : 'docs-nav-item'
}

/**
 * The concept tree, grouped the way the sidebar groups the workspace.
 *
 * Groups fold because twenty concepts listed flat would push the API entries
 * off the first screen, which is the failure mode a sidebar is supposed to fix
 * rather than reproduce. A group opens itself when the concept you are reading
 * is inside it, so you are never on a page hidden in its own navigation.
 */
function ConceptsNav({
  base,
  pathname,
  onNavigate,
}: {
  base: string
  pathname: string
  onNavigate: () => void
}) {
  const groups = conceptNavGroups(base)
  const onIndex = pathname === base || pathname === `${base}/`

  return (
    <>
      <p className="docs-nav-label">Concepts</p>
      <NavLink end to={base} className={navClass} onClick={onNavigate}>
        All concepts
      </NavLink>
      {groups.map((group) => (
        <ConceptGroup
          key={group.id}
          label={group.label}
          hint={group.hint}
          items={group.items}
          pathname={pathname}
          startOpen={onIndex}
          onNavigate={onNavigate}
        />
      ))}
    </>
  )
}

function ConceptGroup({
  label,
  hint,
  items,
  pathname,
  startOpen,
  onNavigate,
}: {
  label: string
  hint: string
  items: { slug: string; title: string; href: string }[]
  pathname: string
  startOpen: boolean
  onNavigate: () => void
}) {
  const hasCurrent = items.some((i) => pathname === i.href)
  const [open, setOpen] = useState(hasCurrent || startOpen)

  useEffect(() => {
    if (hasCurrent) setOpen(true)
  }, [hasCurrent])

  if (items.length === 0) return null

  return (
    <div className="docs-nav-group">
      <button
        type="button"
        className="docs-nav-group-label"
        title={hint}
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <span>{label}</span>
        <span aria-hidden="true" className="nav-caret">
          {open ? '▾' : '▸'}
        </span>
      </button>
      {open &&
        items.map((item) => (
          <NavLink
            key={item.slug}
            to={item.href}
            className={({ isActive }) =>
              isActive ? 'docs-nav-item nested active' : 'docs-nav-item nested'
            }
            onClick={onNavigate}
          >
            {item.title}
          </NavLink>
        ))}
    </div>
  )
}
