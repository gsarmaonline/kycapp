import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  createAppGrant,
  listAppRoles,
  listAppScopeTypes,
  listAppUserGroups,
  listAppUsers,
  type AppRole,
  type AppScopeType,
  type AppUser,
  type AppUserGroup,
} from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'

export function CustomerGrantsNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [roles, setRoles] = useState<AppRole[]>([])
  const [scopeTypes, setScopeTypes] = useState<AppScopeType[]>([])
  const [groups, setGroups] = useState<AppUserGroup[]>([])
  const [customers, setCustomers] = useState<AppUser[]>([])
  const [subjectKind, setSubjectKind] = useState<'app_user' | 'group'>('app_user')
  const [subjectId, setSubjectId] = useState('')
  const [roleId, setRoleId] = useState('')
  const [scopeKind, setScopeKind] = useState('')
  const [scopeId, setScopeId] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([
      listAppRoles(orgId),
      listAppScopeTypes(orgId),
      listAppUserGroups(orgId),
      listAppUsers(orgId),
    ])
      .then(([r, s, g, u]) => {
        setRoles(r.items)
        setScopeTypes(s.items)
        setGroups(g.items)
        setCustomers(u.items)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
  }, [orgId])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await createAppGrant(orgId, {
        app_user_id: subjectKind === 'app_user' ? subjectId : undefined,
        group_id: subjectKind === 'group' ? subjectId : undefined,
        role_id: roleId,
        scope_kind: scopeKind,
        scope_id: scopeId,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      })
      navigate(resourcePath(orgId, 'customer-grants'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Grant failed')
    }
  }

  const missing = roles.length === 0 || scopeTypes.length === 0

  return (
    <section>
      <PageHeader title="Grant access" />
      {error && <p className="error">{error}</p>}
      {missing && (
        <p className="muted">
          A grant needs a <Link to={orgPath(orgId, 'customer-roles')}>role</Link> and a{' '}
          <Link to={orgPath(orgId, 'customer-scope-kinds')}>scope kind</Link>. Create those first.
        </p>
      )}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Give access to
          <select
            value={subjectKind}
            onChange={(e) => {
              setSubjectKind(e.target.value as 'app_user' | 'group')
              setSubjectId('')
            }}
          >
            <option value="app_user">One customer</option>
            <option value="group">A group of customers</option>
          </select>
        </label>
        <label>
          {subjectKind === 'group' ? 'Group' : 'Customer'}
          <select value={subjectId} onChange={(e) => setSubjectId(e.target.value)} required>
            <option value="">Choose…</option>
            {subjectKind === 'group'
              ? groups.map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name} ({g.member_count})
                  </option>
                ))
              : customers.map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.email ?? u.display_name ?? u.id}
                  </option>
                ))}
          </select>
        </label>
        <label>
          Role
          <select value={roleId} onChange={(e) => setRoleId(e.target.value)} required>
            <option value="">Choose…</option>
            {roles.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Scope kind
          <select value={scopeKind} onChange={(e) => setScopeKind(e.target.value)} required>
            <option value="">Choose…</option>
            {scopeTypes.map((s) => (
              <option key={s.id} value={s.kind}>
                {s.label || s.kind}
              </option>
            ))}
          </select>
        </label>
        <label>
          Scope id
          <input
            value={scopeId}
            onChange={(e) => setScopeId(e.target.value)}
            placeholder="the id of the project, region or account"
            required
          />
        </label>
        <label>
          Expires (optional)
          <input type="date" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'customer-grants')} submitLabel="Grant access" />
      </form>
    </section>
  )
}
