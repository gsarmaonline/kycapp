import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  createAppUser,
  listAttributeDefinitions,
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

export function UsersNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [defs, setDefs] = useState<AttributeDefinition[]>([])
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [attrDraft, setAttrDraft] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void listAttributeDefinitions(orgId, 'active')
      .then((d) => setDefs(d.items))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load schema'))
  }, [orgId])

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
      const u = await createAppUser(orgId, {
        email: email || undefined,
        display_name: displayName,
        attributes: coerceAttributes(),
      })
      navigate(resourcePath(orgId, 'users', u.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Create user" />
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
        {groupBySection(defs).map(([sec, items]) => (
          <fieldset key={sec} className="perm-group attr-section">
            <legend>{sec}</legend>
            {items.map((d) => (
              <label key={d.id}>
                {d.label}
                {d.required ? ' *' : ''}
                {d.value_type === 'boolean' || d.value_type === 'dropdown' ? (
                  <select
                    value={attrDraft[d.key] ?? ''}
                    onChange={(e) =>
                      setAttrDraft((prev) => ({ ...prev, [d.key]: e.target.value }))
                    }
                    required={d.required}
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
                    required={d.required}
                  />
                )}
              </label>
            ))}
          </fieldset>
        ))}
        <FormActions cancelTo={resourcePath(orgId, 'users')} submitLabel="Create" />
      </form>
    </section>
  )
}
