import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  createAppGrant,
  deleteAppGrant,
  getAppUserAccess,
  listAppGrants,
  listAppRoles,
  listAppScopeTypes,
  listAppUserGroups,
  listAppUsers,
  listGroupsForAppUser,
  type AppAccessSet,
  type AppUser,
} from '../../api'
import { ResourceTable } from '../../crud/ui'
import { orgPath } from '../../org_nav'
import { AccessHeader, useResource } from './shared'

export function CustomerGrantsPage() {
  const { orgId = '' } = useParams()
  const { data, error, loading, run } = useResource(
    async () => ({
      grants: (await listAppGrants(orgId)).items,
      roles: (await listAppRoles(orgId)).items,
      groups: (await listAppUserGroups(orgId)).items,
      scopeTypes: (await listAppScopeTypes(orgId)).items,
      customers: (await listAppUsers(orgId)).items,
    }),
    [orgId],
  )

  const [subjectKind, setSubjectKind] = useState<'app_user' | 'group'>('app_user')
  const [subjectId, setSubjectId] = useState('')
  const [roleId, setRoleId] = useState('')
  const [scopeKind, setScopeKind] = useState('')
  const [scopeId, setScopeId] = useState('')

  const grants = data?.grants ?? []
  const roles = data?.roles ?? []
  const groups = data?.groups ?? []
  const scopeTypes = data?.scopeTypes ?? []
  const customers = data?.customers ?? []
  const ready = roles.length > 0 && scopeTypes.length > 0

  return (
    <section>
      <AccessHeader title="Grants">
        Who holds which role, and where. Your backend reads the result from{' '}
        <code>GET /v1/app-users/&#123;id&#125;/access</code> and evaluates it locally, so we are not
        in the path of every request in your product.
      </AccessHeader>

      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <>
          {!ready && (
            <p className="muted">
              You need at least one <Link to={orgPath(orgId, 'customer-scope-kinds')}>scope kind</Link>{' '}
              and one <Link to={orgPath(orgId, 'customer-roles')}>role</Link> before granting.
            </p>
          )}
          <form
            className="row wrap"
            onSubmit={(e) => {
              e.preventDefault()
              void run(async () => {
                await createAppGrant(orgId, {
                  ...(subjectKind === 'group' ? { group_id: subjectId } : { app_user_id: subjectId }),
                  role_id: roleId,
                  scope_kind: scopeKind,
                  scope_id: scopeId,
                })
                setSubjectId('')
                setScopeId('')
              })
            }}
          >
            <select
              value={subjectKind}
              onChange={(e) => {
                setSubjectKind(e.target.value as 'app_user' | 'group')
                setSubjectId('')
              }}
              aria-label="Grant to"
            >
              <option value="app_user">A customer</option>
              <option value="group">A group</option>
            </select>
            <select
              value={subjectId}
              onChange={(e) => setSubjectId(e.target.value)}
              aria-label="Subject"
              required
            >
              <option value="">Choose…</option>
              {subjectKind === 'group'
                ? groups.map((g) => (
                    <option key={g.id} value={g.id}>
                      {g.name}
                    </option>
                  ))
                : customers.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.email ?? u.display_name ?? u.id}
                    </option>
                  ))}
            </select>
            <select value={roleId} onChange={(e) => setRoleId(e.target.value)} aria-label="Role" required>
              <option value="">Role…</option>
              {roles.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </select>
            <select
              value={scopeKind}
              onChange={(e) => setScopeKind(e.target.value)}
              aria-label="Scope kind"
              required
            >
              <option value="">Scope kind…</option>
              {scopeTypes.map((s) => (
                <option key={s.id} value={s.kind}>
                  {s.kind}
                </option>
              ))}
            </select>
            <input
              value={scopeId}
              onChange={(e) => setScopeId(e.target.value)}
              placeholder="your id, e.g. p1"
              aria-label="Scope id"
              required
            />
            <button type="submit" disabled={!ready}>
              Grant
            </button>
          </form>

          <ResourceTable
            columns={['Subject', 'Role', 'Scope']}
            empty="No grants yet."
            rows={grants.map((g) => ({
              key: g.id,
              cells: [
                <span key="s">
                  {g.subject_kind === 'group' ? 'Group ' : ''}
                  <strong>{g.subject_label || '—'}</strong>
                </span>,
                <code key="r">{g.role_key}</code>,
                <code key="sc">
                  {g.scope_kind}:{g.scope_id}
                </code>,
              ],
              actions: (
                <button type="button" onClick={() => void run(() => deleteAppGrant(orgId, g.id))}>
                  Revoke
                </button>
              ),
            }))}
          />

          <CustomerLookup customers={customers} />
        </>
      )}
    </section>
  )
}

/**
 * Answers "why does this customer have this access?".
 *
 * A grant list tells you what was configured; this tells you what one customer
 * ends up with, and where each capability came from. It lives beside grants
 * because that is the question you have while looking at them.
 */
function CustomerLookup({ customers }: { customers: AppUser[] }) {
  const [selected, setSelected] = useState('')
  const [groups, setGroups] = useState<{ id: string; key: string; name: string }[]>([])
  const [access, setAccess] = useState<AppAccessSet | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!selected) {
      setGroups([])
      setAccess(null)
      return
    }
    let cancelled = false
    setError(null)
    Promise.all([listGroupsForAppUser(selected), getAppUserAccess(selected)])
      .then(([g, a]) => {
        if (cancelled) return
        setGroups(g.items)
        setAccess(a)
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load')
      })
    return () => {
      cancelled = true
    }
  }, [selected])

  return (
    <div className="stack">
      <h2>Check one customer</h2>
      <p className="muted">Exactly what your backend receives for them, and where each part came from.</p>
      <select value={selected} onChange={(e) => setSelected(e.target.value)} aria-label="Customer">
        <option value="">Choose a customer…</option>
        {customers.map((u) => (
          <option key={u.id} value={u.id}>
            {u.email ?? u.display_name ?? u.id}
          </option>
        ))}
      </select>

      {error && <p className="error">{error}</p>}
      {selected && (
        <>
          <p className="muted">
            Groups: {groups.length ? groups.map((g) => g.name).join(', ') : 'none'}
            {access ? ` · version ${access.version}` : ''}
          </p>
          <ResourceTable
            columns={['Scope', 'Capabilities', 'Via']}
            empty="No access. Grant a role, directly or through a group."
            rows={(access?.grants ?? []).map((g) => ({
              key: g.id,
              cells: [
                <code key="s">
                  {g.scope_kind}:{g.scope_id}
                </code>,
                <div className="row wrap" key="c">
                  {g.capabilities.map((c) => (
                    <code key={c}>{c}</code>
                  ))}
                </div>,
                <span key="v" className="muted">
                  {g.source}
                </span>,
              ],
            }))}
          />
        </>
      )}
    </div>
  )
}
