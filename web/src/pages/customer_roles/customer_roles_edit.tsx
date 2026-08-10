import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  getAppRole, updateAppRole,
  listAppCapabilities,
  listAppRoles,
  type AppCapability,
  type AppRole,
} from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'

export function CustomerRolesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [capabilities, setCapabilities] = useState<AppCapability[]>([])
  const [roles, setRoles] = useState<AppRole[]>([])
  const [picked, setPicked] = useState<string[]>([])
  const [extendsIds, setExtendsIds] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([listAppCapabilities(orgId), listAppRoles(orgId), getAppRole(orgId, id)])
      .then(([caps, list, role]) => {
        setCapabilities(caps.items)
        setRoles(list.items.filter((r) => r.id !== id))
        setKey(role.key)
        setName(role.name)
        setPicked(role.own_capabilities)
        setExtendsIds(role.extends ?? [])
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
  }, [orgId, id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateAppRole(orgId, id, { name, capabilities: picked, extends: extendsIds })
      navigate(resourcePath(orgId, 'customer-roles'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <section>
      <PageHeader title="Edit role" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="maintainer"
            disabled
          />
        </label>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Maintainer" />
        </label>
        <fieldset className="perm-group">
          <legend>Capabilities this role adds</legend>
          {capabilities.length === 0 ? (
            <p className="muted">
              None declared yet.{' '}
              <Link to={orgPath(orgId, 'customer-capabilities')}>Add one</Link> first.
            </p>
          ) : (
            capabilities.map((c) => (
              <label className="perm" key={c.id}>
                <input
                  type="checkbox"
                  checked={picked.includes(c.key)}
                  onChange={(e) =>
                    setPicked((prev) =>
                      e.target.checked ? [...prev, c.key] : prev.filter((k) => k !== c.key),
                    )
                  }
                />
                <span>
                  {c.key}
                  {c.description && <em>{c.description}</em>}
                </span>
              </label>
            ))
          )}
        </fieldset>
        <fieldset className="perm-group">
          <legend>Builds on</legend>
          <p className="muted">
            Capabilities from these roles are included automatically, and stay included when you
            change them later.
          </p>
          {roles.length === 0 ? (
            <p className="muted">No other roles yet.</p>
          ) : (
            roles.map((r) => (
              <label className="perm" key={r.id}>
                <input
                  type="checkbox"
                  checked={extendsIds.includes(r.id)}
                  onChange={(e) =>
                    setExtendsIds((prev) =>
                      e.target.checked ? [...prev, r.id] : prev.filter((x) => x !== r.id),
                    )
                  }
                />
                <span>
                  {r.name}
                  <em>{r.effective_capabilities.join(', ') || 'no capabilities'}</em>
                </span>
              </label>
            ))
          )}
        </fieldset>
        <FormActions cancelTo={resourcePath(orgId, 'customer-roles')} submitLabel="Save" />
      </form>
    </section>
  )
}
