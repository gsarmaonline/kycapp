import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  createAppRole,
  deleteAppRole,
  listAppCapabilities,
  listAppRoles,
  type AppRole,
} from '../../api'
import { ResourceTable } from '../../crud/ui'
import { orgPath } from '../../org_nav'
import { AccessHeader, useResource } from './shared'

export function CustomerRolesPage() {
  const { orgId = '' } = useParams()
  const { data, error, loading, run } = useResource(
    async () => ({
      roles: (await listAppRoles(orgId)).items,
      capabilities: (await listAppCapabilities(orgId)).items,
    }),
    [orgId],
  )
  const [key, setKey] = useState('')
  const [picked, setPicked] = useState<string[]>([])
  const [extendsIds, setExtendsIds] = useState<string[]>([])

  const roles = data?.roles ?? []
  const capabilities = data?.capabilities ?? []

  return (
    <section>
      <AccessHeader title="Roles">
        Named sets of capabilities. A role may build on others, and editing it reaches everyone
        holding it, which is the reason to build a role rather than repeat a list of capabilities.
      </AccessHeader>

      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <>
          <form
            className="stack"
            onSubmit={(e) => {
              e.preventDefault()
              void run(async () => {
                await createAppRole(orgId, { key, capabilities: picked, extends: extendsIds })
                setKey('')
                setPicked([])
                setExtendsIds([])
              })
            }}
          >
            <div className="row">
              <input
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder="maintainer"
                aria-label="Role key"
                required
              />
              <button type="submit">Create role</button>
            </div>

            <fieldset>
              <legend>Capabilities this role adds</legend>
              {capabilities.length === 0 ? (
                <p className="muted">
                  None declared yet. <Link to={orgPath(orgId, 'customer-capabilities')}>Add one</Link>{' '}
                  before building a role.
                </p>
              ) : (
                <div className="row wrap">
                  {capabilities.map((c) => (
                    <label key={c.id}>
                      <input
                        type="checkbox"
                        checked={picked.includes(c.key)}
                        onChange={(e) =>
                          setPicked((prev) =>
                            e.target.checked ? [...prev, c.key] : prev.filter((k) => k !== c.key),
                          )
                        }
                      />{' '}
                      <code>{c.key}</code>
                    </label>
                  ))}
                </div>
              )}
            </fieldset>

            <fieldset>
              <legend>Builds on</legend>
              <p className="muted">
                Capabilities from these roles are included automatically, and stay included when you
                change them later.
              </p>
              {roles.length === 0 ? (
                <p className="muted">No roles to build on yet.</p>
              ) : (
                <div className="row wrap">
                  {roles.map((r) => (
                    <label key={r.id}>
                      <input
                        type="checkbox"
                        checked={extendsIds.includes(r.id)}
                        onChange={(e) =>
                          setExtendsIds((prev) =>
                            e.target.checked ? [...prev, r.id] : prev.filter((id) => id !== r.id),
                          )
                        }
                      />{' '}
                      {r.name}
                    </label>
                  ))}
                </div>
              )}
            </fieldset>
          </form>

          <ResourceTable
            columns={['Role', 'Own', 'Effective']}
            empty="No roles yet."
            rows={roles.map((r) => ({
              key: r.id,
              cells: [
                <div key="n">
                  <strong>{r.name}</strong>
                  {r.key !== r.name && (
                    <>
                      <br />
                      <code className="muted">{r.key}</code>
                    </>
                  )}
                </div>,
                <CapList key="o" items={r.own_capabilities} />,
                // Showing only what a role declares is how inheritance
                // surprises people, so the resolved set sits beside it with
                // inherited entries marked.
                <CapList key="e" items={r.effective_capabilities} role={r} />,
              ],
              actions: (
                <button type="button" onClick={() => void run(() => deleteAppRole(orgId, r.id))}>
                  Delete
                </button>
              ),
            }))}
          />
        </>
      )}
    </section>
  )
}

function CapList({ items, role }: { items: string[]; role?: AppRole }) {
  if (items.length === 0) return <span className="muted">none</span>
  const own = new Set(role?.own_capabilities ?? [])
  return (
    <div className="row wrap">
      {items.map((k) => {
        const inherited = role && !own.has(k)
        return (
          <code key={k} title={inherited ? 'Inherited from a role this builds on' : undefined}>
            {k}
            {inherited ? ' ↑' : ''}
          </code>
        )
      })}
    </div>
  )
}
