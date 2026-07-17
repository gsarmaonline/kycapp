import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import {
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
import { ORG_SECTIONS, orgPath, sectionFromPathname } from './org_nav'

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
      if (routeOrgId && items.length > 0 && !items.some((o) => o.id === routeOrgId)) {
        navigate(items[0] ? orgPath(items[0].id) : '/', { replace: true })
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

  return (
    <div className="shell">
      <aside className="sidebar" aria-label="Organisation navigation">
        <div className="sidebar-brand">
          <p className="eyebrow">KYC</p>
          <strong>Organisations</strong>
        </div>

        <label className="org-switcher">
          <span>Current organisation</span>
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
        </label>

        {routeOrgId && (
          <nav className="sidebar-nav" aria-label="Organisation sections">
            {ORG_SECTIONS.map((item) => (
              <NavLink
                key={item.id}
                to={orgPath(routeOrgId, item.id)}
                className={({ isActive }) => (isActive ? 'nav-item active' : 'nav-item')}
                end={item.id === 'overview'}
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
        )}

        <div className="sidebar-actions">
          <button type="button" className="ghost full" onClick={() => setCreating((v) => !v)}>
            {creating ? 'Cancel' : 'New organisation'}
          </button>
          {creating && (
            <form className="create-org" onSubmit={onCreateOrg}>
              <input
                value={newOrgName}
                onChange={(e) => setNewOrgName(e.target.value)}
                placeholder="Organisation name"
                required
              />
              <button type="submit">Create</button>
            </form>
          )}
        </div>

        <div className="sidebar-footer">
          {user && <p className="sidebar-user">{user.email}</p>}
          <button type="button" className="ghost full" onClick={() => void onLogout()}>
            Sign out
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
            <h1>Your organisations</h1>
            <p className="lede">Create an organisation to invite teammates and manage access.</p>
          </div>
        )}
        {selected && (
          <div className="workspace">
            <header className="workspace-header">
              <div>
                <p className="eyebrow">{org?.slug ?? selected.slug}</p>
                <h1>{org?.name ?? selected.name}</h1>
                <p className="status">
                  {org?.status ?? selected.status}
                  {user ? ` · ${user.email}` : ''}
                </p>
              </div>
            </header>
            <Outlet />
          </div>
        )}
      </main>
    </div>
  )
}
