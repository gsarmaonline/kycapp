import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  getAppUser,
  listAttributeDefinitions,
  updateAppUser,
  type AttributeDefinition,
} from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

function groupBySection(defs: AttributeDefinition[]) {
  const map = new Map<string, AttributeDefinition[]>()
  for (const d of defs) {
    const list = map.get(d.section) ?? []
    list.push(d)
    map.set(d.section, list)
  }
  return [...map.entries()]
}

export function UsersEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [defs, setDefs] = useState<AttributeDefinition[]>([])
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [status, setStatus] = useState('active')
  const [attrDraft, setAttrDraft] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void (async () => {
      try {
        const [u, d] = await Promise.all([
          getAppUser(id),
          listAttributeDefinitions(orgId, 'active'),
        ])
        setDefs(d.items)
        setDisplayName(u.display_name || '')
        setEmail(u.email || '')
        setStatus(u.status)
        const draft: Record<string, string> = {}
        for (const [k, v] of Object.entries(u.attributes || {})) {
          draft[k] = String(v)
        }
        setAttrDraft(draft)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load')
      } finally {
        setLoading(false)
      }
    })()
  }, [id, orgId])

  function coerceAttributes(): Record<string, unknown> {
    const out: Record<string, unknown> = {}
    for (const d of defs) {
      const raw = attrDraft[d.key]
      if (raw === undefined || raw === '') continue
      switch (d.value_type) {
        case 'number':
          out[d.key] = Number(raw)
          break
        case 'boolean':
          out[d.key] = raw === 'true'
          break
        default:
          out[d.key] = raw
      }
    }
    return out
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateAppUser(id, {
        display_name: displayName,
        email,
        status,
        attributes: coerceAttributes(),
      })
      navigate(resourcePath(orgId, 'users', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit user" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Display name
          <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </label>
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </label>
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="active">active</option>
            <option value="disabled">disabled</option>
            <option value="archived">archived</option>
          </select>
        </label>
        {groupBySection(defs).map(([sec, items]) => (
          <fieldset key={sec} className="perm-group attr-section">
            <legend>{sec}</legend>
            {items.map((d) => (
              <label key={d.id}>
                {d.label}
                {d.value_type === 'dropdown' || d.value_type === 'boolean' ? (
                  <select
                    value={attrDraft[d.key] ?? ''}
                    onChange={(e) =>
                      setAttrDraft((prev) => ({ ...prev, [d.key]: e.target.value }))
                    }
                  >
                    <option value="">—</option>
                    {d.value_type === 'boolean' ? (
                      <>
                        <option value="true">true</option>
                        <option value="false">false</option>
                      </>
                    ) : (
                      d.enum_values.map((v) => (
                        <option key={v} value={v}>
                          {v}
                        </option>
                      ))
                    )}
                  </select>
                ) : (
                  <input
                    type={
                      d.value_type === 'number' ? 'number' : d.value_type === 'date' ? 'date' : 'text'
                    }
                    value={attrDraft[d.key] ?? ''}
                    onChange={(e) =>
                      setAttrDraft((prev) => ({ ...prev, [d.key]: e.target.value }))
                    }
                  />
                )}
              </label>
            ))}
          </fieldset>
        ))}
        <FormActions cancelTo={resourcePath(orgId, 'users', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
