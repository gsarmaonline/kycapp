import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { NavLink, useNavigate, useParams } from 'react-router-dom'
import {
  createOrganisation,
  getOrganisation,
  listMemberships,
  listOrganisations,
  listPermissions,
  listRoles,
  type Membership,
  type Organisation,
  type Permission,
  type Role,
  type User,
} from './api'
import { ORG_SECTIONS, orgPath, sectionFromParam } from './org_nav'
import type { OrgSection } from './org_nav'
import { OverviewPanel } from './panels/overview_panel'
import { MembersPanel } from './panels/members_panel'
import { RolesPanel } from './panels/roles_panel'
import { UsersPanel } from './panels/users_panel'
import { AttributesPanel } from './panels/attributes_panel'
import { EmailTemplatesPanel } from './panels/email_templates_panel'
import { BillingPanel } from './panels/billing_panel'

export function AppShell({ user, onLogout }: { user: User | null; onLogout: () => Promise<void> }) {
  const { orgId: routeOrgId, section: routeSection } = useParams()
  const navigate = useNavigate()
  const section = sectionFromParam(routeSection)

  const [orgs, setOrgs] = useState<Organisation[]>([])
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
        navigate(items[0] ? orgPath(items[0].id) : '/orgs', { replace: true })
      }
    })()
  }, [routeOrgId])

  function switchOrg(id: string) {
    navigate(orgPath(id, section === 'overview' ? 'overview' : section))
  }

  async function onCreateOrg(e: FormEvent) {
    e.preventDefault()
    setOrgsError(null)
    try {
      const org = await createOrganisation(newOrgName)
      setNewOrgName('')
      setCreating(false)
      await refreshOrgs()
      navigate(orgPath(org.id, 'overview'))
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
          <OrgWorkspace
            key={selected.id}
            orgId={selected.id}
            section={section}
            user={user}
          />
        )}
      </main>
    </div>
  )
}

function OrgWorkspace({
  orgId,
  section,
  user,
}: {
  orgId: string
  section: OrgSection
  user: User | null
}) {
  const [org, setOrg] = useState<Organisation | null>(null)
  const [members, setMembers] = useState<Membership[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setError(null)
    setLoading(true)
    try {
      const [o, m, r, p] = await Promise.all([
        getOrganisation(orgId),
        listMemberships(orgId),
        listRoles(orgId),
        listPermissions(),
      ])
      setOrg(o)
      setMembers(m.items)
      setRoles(r.items)
      setPermissions(p.items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  const title = ORG_SECTIONS.find((s) => s.id === section)?.label ?? 'Overview'

  return (
    <div className="workspace">
      <header className="workspace-header">
        <div>
          <p className="eyebrow">{org?.slug ?? '…'}</p>
          <h1>{org?.name ?? 'Organisation'}</h1>
          <p className="status">
            {org?.status ?? '…'}
            {user ? ` · ${user.email}` : ''}
          </p>
        </div>
        <h2 className="section-title">{title}</h2>
      </header>

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {loading && <p>Loading…</p>}

      {!loading && section === 'overview' && org && (
        <OverviewPanel org={org} memberCount={members.length} roleCount={roles.length} />
      )}
      {!loading && section === 'members' && (
        <MembersPanel
          orgId={orgId}
          members={members}
          roles={roles}
          onChanged={refresh}
          onError={setError}
        />
      )}
      {!loading && section === 'roles' && (
        <RolesPanel
          orgId={orgId}
          roles={roles}
          permissions={permissions}
          onChanged={refresh}
          onError={setError}
        />
      )}
      {!loading && section === 'users' && (
        <UsersPanel orgId={orgId} onError={setError} />
      )}
      {!loading && section === 'attributes' && (
        <AttributesPanel orgId={orgId} onError={setError} />
      )}
      {!loading && section === 'email-templates' && (
        <EmailTemplatesPanel orgId={orgId} onError={setError} />
      )}
      {!loading && section === 'billing' && <BillingPanel orgId={orgId} onError={setError} />}
    </div>
  )
}
