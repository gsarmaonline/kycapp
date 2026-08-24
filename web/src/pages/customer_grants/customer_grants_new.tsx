import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  createAppGrant,
  listAppCapabilities,
  listAppRoles,
  listAppScopeTypes,
  listAppUserGroups,
  listAppUsers,
  type AppCapability,
  type AppRole,
  type AppScopeRef,
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
  const [capabilities, setCapabilities] = useState<AppCapability[]>([])

  const [subjectKind, setSubjectKind] = useState<SubjectKind>('app_user')
  const [subjectId, setSubjectId] = useState('')
  const [exceptUsers, setExceptUsers] = useState<string[]>([])

  const [allCapabilities, setAllCapabilities] = useState(false)
  const [roleId, setRoleId] = useState('')
  const [exceptCapabilities, setExceptCapabilities] = useState<string[]>([])

  const [allScopes, setAllScopes] = useState(false)
  const [scopeKind, setScopeKind] = useState('')
  const [scopeId, setScopeId] = useState('')
  const [exceptScopes, setExceptScopes] = useState<AppScopeRef[]>([])
  const [exceptKind, setExceptKind] = useState('')
  const [exceptId, setExceptId] = useState('')

  const [selfOnly, setSelfOnly] = useState(false)
  const [expiresAt, setExpiresAt] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([
      listAppRoles(orgId),
      listAppScopeTypes(orgId),
      listAppUserGroups(orgId),
      listAppUsers(orgId),
      listAppCapabilities(orgId),
    ])
      .then(([r, s, g, u, c]) => {
        setRoles(r.items)
        setScopeTypes(s.items)
        setGroups(g.items)
        setCustomers(u.items)
        setCapabilities(c.items)
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
        except_capabilities: allCapabilities ? exceptCapabilities : [],
        all_scopes: allScopes,
        scope_kind: allScopes ? undefined : scopeKind,
        scope_id: scopeId,
        except_scopes: exceptScopes,
        except_app_user_ids: subjectKind === 'app_user' ? [] : exceptUsers,
        constraint: selfOnly ? 'self_subject' : '',
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      })
      navigate(resourcePath(orgId, 'customer-grants'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Grant failed')
    }
  }

  function toggle(list: string[], value: string, on: boolean): string[] {
    return on ? [...list, value] : list.filter((v) => v !== value)
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
              setExceptUsers([])
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

        {subjectKind !== 'app_user' && customers.length > 0 && (
          <fieldset className="perm-group">
            <legend>Except these customers</legend>
            <p className="muted">
              Offboard one person without listing everyone else. Only this grant is affected.
            </p>
            {customers.map((u) => (
              <label className="perm" key={u.id}>
                <input
                  type="checkbox"
                  checked={exceptUsers.includes(u.id)}
                  onChange={(e) => setExceptUsers(toggle(exceptUsers, u.id, e.target.checked))}
                />
                <span>{u.email ?? u.display_name ?? u.id}</span>
              </label>
            ))}
          </fieldset>
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
                setExceptCapabilities([])
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

          {allCapabilities && capabilities.length > 0 && (
            <>
              <p className="muted">Except these capabilities:</p>
              {capabilities.map((c) => (
                <label className="perm" key={c.id}>
                  <input
                    type="checkbox"
                    checked={exceptCapabilities.includes(c.key)}
                    onChange={(e) =>
                      setExceptCapabilities(toggle(exceptCapabilities, c.key, e.target.checked))
                    }
                  />
                  <span>
                    {c.key}
                    {c.description && <em>{c.description}</em>}
                  </span>
                </label>
              ))}
            </>
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

        <fieldset className="perm-group">
          <legend>Except these scopes</legend>
          <p className="muted">
            For what narrower scoping cannot say: ten thousand projects, one confidential, and no
            appetite for 9,999 grants. Another grant reaching the same scope still allows it.
          </p>
          {exceptScopes.length > 0 && (
            <ul>
              {exceptScopes.map((s, i) => (
                <li key={`${s.kind}/${s.id}`}>
                  {s.kind} / {s.id}{' '}
                  <button
                    type="button"
                    className="link-btn danger"
                    onClick={() => setExceptScopes(exceptScopes.filter((_, j) => j !== i))}
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
          )}
          <div className="row">
            <select
              value={exceptKind}
              onChange={(e) => setExceptKind(e.target.value)}
              aria-label="Excluded scope kind"
            >
              <option value="">Kind…</option>
              {scopeTypes.map((s) => (
                <option key={s.id} value={s.kind}>
                  {s.label || s.kind}
                </option>
              ))}
            </select>
            <input
              value={exceptId}
              onChange={(e) => setExceptId(e.target.value)}
              placeholder="id to exclude"
              aria-label="Excluded scope id"
            />
            <button
              type="button"
              disabled={!exceptKind || !exceptId}
              onClick={() => {
                setExceptScopes([...exceptScopes, { kind: exceptKind, id: exceptId }])
                setExceptId('')
              }}
            >
              Exclude
            </button>
          </div>
        </fieldset>

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
