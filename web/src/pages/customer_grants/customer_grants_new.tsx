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

type SubjectKind = 'app_user' | 'group' | 'everyone'

/**
 * A grant names a subject, what it carries, and where it reaches. Two of those
 * three can be a wildcard, and each wildcard has an exclusion list beside it:
 * a wildcard says "I cannot list this set", an exclusion says "but I know
 * members that do not belong".
 *
 * Every exclusion narrows this grant alone. None of them is a deny rule, so
 * another grant reaching the same resource still allows it.
 */
export function CustomerGrantsNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [roles, setRoles] = useState<AppRole[]>([])
  const [scopeTypes, setScopeTypes] = useState<AppScopeType[]>([])
  const [groups, setGroups] = useState<AppUserGroup[]>([])
  const [customers, setCustomers] = useState<AppUser[]>([])

  const [subjectKind, setSubjectKind] = useState<SubjectKind>('app_user')
  const [subjectId, setSubjectId] = useState('')

  const [allCapabilities, setAllCapabilities] = useState(false)
  const [roleId, setRoleId] = useState('')

  const [allScopes, setAllScopes] = useState(false)
  const [scopeKind, setScopeKind] = useState('')
  const [scopeId, setScopeId] = useState('')

  const [selfOnly, setSelfOnly] = useState(false)
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
        subject_kind: subjectKind,
        app_user_id: subjectKind === 'app_user' ? subjectId : undefined,
        group_id: subjectKind === 'group' ? subjectId : undefined,
        role_id: allCapabilities ? undefined : roleId,
        all_capabilities: allCapabilities,
        all_scopes: allScopes,
        scope_kind: allScopes ? undefined : scopeKind,
        scope_id: scopeId,
        constraint: selfOnly ? 'self_subject' : '',
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      })
      navigate(resourcePath(orgId, 'customer-grants'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Grant failed')
    }
  }

  const missing = scopeTypes.length === 0 || (roles.length === 0 && !allCapabilities)

  return (
    <section>
      <PageHeader title="Grant access" />
      {error && <p className="error">{error}</p>}
      {missing && (
        <p className="muted">
          A grant needs a <Link to={orgPath(orgId, 'customer-scope-kinds')}>scope kind</Link>, and a{' '}
          <Link to={orgPath(orgId, 'customer-roles')}>role</Link> unless it carries every
          capability. Create those first.
        </p>
      )}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Give access to
          <select
            value={subjectKind}
            onChange={(e) => {
              setSubjectKind(e.target.value as SubjectKind)
              setSubjectId('')
            }}
          >
            <option value="app_user">One customer</option>
            <option value="group">A group of customers</option>
            <option value="everyone">Every customer</option>
          </select>
        </label>

        {subjectKind !== 'everyone' && (
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
        )}

        {subjectKind === 'everyone' && (
          <p className="muted">
            Covers every customer of this organisation, including ones who sign up later. One row,
            however many customers you have.
          </p>
        )}

        <fieldset className="perm-group">
          <legend>What it carries</legend>
          <label className="perm">
            <input
              type="checkbox"
              checked={allCapabilities}
              onChange={(e) => {
                setAllCapabilities(e.target.checked)
                setRoleId('')
              }}
            />
            <span>
              Every capability
              <em>
                Including capabilities you declare later. That is the point, and also the risk: a
                new capability reaches everyone holding this grant without anyone editing it.
              </em>
            </span>
          </label>

          {!allCapabilities && (
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
          )}

        </fieldset>

        {/*
          The organisation is the ceiling, never a scope kind: a grant already
          lives in exactly one, so declaring it would be a second way to say
          what every grant already says.
        */}
        <label className="inline-check">
          <input
            type="checkbox"
            checked={allScopes}
            onChange={(e) => setAllScopes(e.target.checked)}
          />
          Everywhere in this organisation
        </label>

        {!allScopes && (
          <>
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
              <span className="field-hint">
                Use <code>*</code> for every {scopeKind || 'one'} you have now or add later.
              </span>
            </label>
          </>
        )}


        <label className="perm">
          <input
            type="checkbox"
            checked={selfOnly}
            onChange={(e) => setSelfOnly(e.target.checked)}
          />
          <span>
            Only their own resources
            <em>
              Applies the grant to rows belonging to the holder. Your backend enforces this: it
              must set the subject on the grant set and on the resource, or the restriction is
              lost.
            </em>
          </span>
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
