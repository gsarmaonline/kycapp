import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import {
  Link,
  NavLink,
  Outlet,
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom'
import {
  createOrganisation,
  getOrganisation,
  listOrganisations,
  type Organisation,
  type User,
} from './api'
import { ORG_NAV_GROUPS, orgPath, sectionFromPathname } from './org_nav'

function userInitials(label: string): string {
  const parts = label.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

export function AppShell({ user, onLogout }: { user: User | null; onLogout: () => Promise<void> }) {
  const { orgId: routeOrgId } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const section = routeOrgId
    ? sectionFromPathname(location.pathname, routeOrgId)
    : 'overview'

  const [orgs, setOrgs] = useState<Organisation[]>([])
  const [org, setOrg] = useState<Organisation | null>(null)
  const [orgsError, setOrgsError] = useState<string | null>(null)
  const [orgsLoading, setOrgsLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [newOrgName, setNewOrgName] = useState('')

  const selected = orgs.find((o) => o.id === routeOrgId) ?? null

  async function refreshOrgs() {
    setOrgsLoading(true)
    setOrgsError(null)
    try {
      const res = await listOrganisations()
      setOrgs(res.items)
      return res.items
    } catch (e) {
      setOrgsError(e instanceof Error ? e.message : 'Failed to load organisations')
      return [] as Organisation[]
    } finally {
      setOrgsLoading(false)
    }
  }

  useEffect(() => {
    void (async () => {
      const items = await refreshOrgs()
      if (!routeOrgId && items[0]) {
        navigate(orgPath(items[0].id, 'overview'), { replace: true })
        return
      }
      if (routeOrgId && !items.some((o) => o.id === routeOrgId)) {
        navigate(items[0] ? orgPath(items[0].id) : '/app', { replace: true })
      }
    })()
  }, [routeOrgId])

  useEffect(() => {
    if (!routeOrgId) {
      setOrg(null)
      return
    }
    void getOrganisation(routeOrgId)
      .then(setOrg)
      .catch(() => setOrg(null))
  }, [routeOrgId])

  function switchOrg(id: string) {
    navigate(orgPath(id, section === 'overview' ? 'overview' : section))
  }

  async function onCreateOrg(e: FormEvent) {
    e.preventDefault()
    setOrgsError(null)
    try {
      const created = await createOrganisation(newOrgName)
      setNewOrgName('')
      setCreating(false)
      await refreshOrgs()
      navigate(orgPath(created.id, 'overview'))
    } catch (err) {
      setOrgsError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  const noOrgs = !orgsLoading && orgs.length === 0
  const showCreateInMain = noOrgs || creating

  return (
    <div className="shell">
      <aside className="sidebar" aria-label="Organisation navigation">
        <div className="sidebar-brand">
          <Link to="/" className="sidebar-brand-link eyebrow">
            KYC
          </Link>
        </div>

        {routeOrgId && selected && (
          <nav className="sidebar-nav" aria-label="Organisation sections">
            <NavLink
              to={orgPath(routeOrgId, 'overview')}
              className={({ isActive }) => (isActive ? 'nav-item active' : 'nav-item')}
              end
            >
              Overview
            </NavLink>
            {ORG_NAV_GROUPS.map((group) => (
              <div key={group.id} className="sidebar-nav-group">
                <p className="sidebar-nav-group-label" title={group.hint}>
                  {group.label}
                </p>
                {group.items.map((item) => (
                  <NavLink
                    key={item.id}
                    to={orgPath(routeOrgId, item.id)}
                    className={({ isActive }) => (isActive ? 'nav-item active' : 'nav-item')}
                  >
                    {item.label}
                  </NavLink>
                ))}
              </div>
            ))}
          </nav>
        )}

        <div className="sidebar-footer">
          <div className="org-switcher">
            <span>Current organisation</span>
            <div className="org-switcher-row">
              <select
                value={routeOrgId ?? ''}
                disabled={orgsLoading || orgs.length === 0}
                onChange={(e) => switchOrg(e.target.value)}
                aria-label="Switch organisation"
              >
                {orgs.length === 0 && <option value="">No organisations</option>}
                {orgs.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="org-add-btn"
                aria-label={creating ? 'Cancel new organisation' : 'New organisation'}
                title={creating ? 'Cancel' : 'New organisation'}
                aria-pressed={creating}
                onClick={() => setCreating((v) => !v)}
              >
                {creating ? '×' : '+'}
              </button>
            </div>
            {creating && !noOrgs && (
              <form className="create-org" onSubmit={onCreateOrg}>
                <input
                  value={newOrgName}
                  onChange={(e) => setNewOrgName(e.target.value)}
                  placeholder="Organisation name"
                  required
                  autoFocus
                />
                <button type="submit">Create</button>
              </form>
            )}
          </div>

          {user && (
            <div className="sidebar-account">
              {user.avatar_url ? (
                <img
                  className="sidebar-avatar"
                  src={user.avatar_url}
                  alt=""
                  referrerPolicy="no-referrer"
                />
              ) : (
                <span className="sidebar-avatar sidebar-avatar-fallback" aria-hidden="true">
                  {userInitials(user.name || user.email)}
                </span>
              )}
              <div className="sidebar-account-text">
                <strong className="sidebar-user-name">{user.name || user.email}</strong>
                <p className="sidebar-user">{user.email}</p>
              </div>
            </div>
          )}
          <button type="button" className="ghost full" onClick={() => void onLogout()}>
            Log out
          </button>
        </div>
      </aside>

      <main className="main">
        {orgsError && (
          <p className="error" role="alert">
            {orgsError}
          </p>
        )}
        {orgsLoading && <p>Loading organisations…</p>}
        {!orgsLoading && !selected && (
          <div className="empty-state">
            <h1>{noOrgs ? 'Create your organisation' : 'Your organisations'}</h1>
            <p className="lede">
              {noOrgs
                ? 'Organisations are the hub for members, end users, billing, and product features.'
                : 'This organisation is unavailable (it may have been deleted). Select another from the sidebar or create a new one.'}
            </p>
            {showCreateInMain && (
              <form className="create-org create-org-main" onSubmit={onCreateOrg}>
                <label>
                  Organisation name
                  <input
                    value={newOrgName}
                    onChange={(e) => setNewOrgName(e.target.value)}
                    placeholder="Acme"
                    required
                    autoFocus={noOrgs}
                  />
                </label>
                <button type="submit">Create organisation</button>
              </form>
            )}
          </div>
        )}
        {selected && (
          <div className="workspace">
            <header className="workspace-header">
              <div>
                <p className="eyebrow">{org?.slug ?? selected.slug}</p>
                <h1>{org?.name ?? selected.name}</h1>
                <p className="status">{org?.status ?? selected.status}</p>
              </div>
            </header>
            <Outlet />
          </div>
        )}
      </main>
    </div>
  )
}
