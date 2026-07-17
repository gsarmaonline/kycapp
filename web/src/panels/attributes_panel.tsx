import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import {
  createAttributeDefinition,
  listAttributeDefinitions,
  type AttributeDefinition,
} from '../api'

export function groupDefsBySection(defs: AttributeDefinition[]) {
  const map = new Map<string, AttributeDefinition[]>()
  for (const d of defs) {
    const list = map.get(d.section) ?? []
    list.push(d)
    map.set(d.section, list)
  }
  return [...map.entries()]
}

export function AttributesPanel({
  orgId,
  onError,
}: {
  orgId: string
  onError: (msg: string | null) => void
}) {
  const [defs, setDefs] = useState<AttributeDefinition[]>([])
  const [key, setKey] = useState('')
  const [label, setLabel] = useState('')
  const [section, setSection] = useState('general')
  const [valueType, setValueType] = useState('string')
  const [required, setRequired] = useState(false)
  const [enumValues, setEnumValues] = useState('')

  async function refresh() {
    onError(null)
    try {
      const res = await listAttributeDefinitions(orgId)
      setDefs(res.items)
    } catch (err) {
      onError(err instanceof Error ? err.message : 'User attributes load failed')
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  const needsOptions = valueType === 'dropdown'

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    onError(null)
    try {
      await createAttributeDefinition(orgId, {
        key,
        label,
        section: section || 'general',
        value_type: valueType,
        required,
        enum_values: needsOptions
          ? enumValues
              .split(',')
              .map((v) => v.trim())
              .filter(Boolean)
          : undefined,
      })
      setKey('')
      setLabel('')
      setEnumValues('')
      await refresh()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Create attribute failed')
    }
  }

  const grouped = groupDefsBySection(defs)

  return (
    <section className="user-attributes">
      <p className="lede">
        Define profile fields for end users. Use <em>section</em> to group fields in forms.
      </p>
      <form className="create" onSubmit={onCreate}>
        <label>
          Key
          <input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder="country"
            required
          />
        </label>
        <label>
          Label
          <input value={label} onChange={(e) => setLabel(e.target.value)} required />
        </label>
        <label>
          Section
          <input value={section} onChange={(e) => setSection(e.target.value)} placeholder="general" />
        </label>
        <label>
          Type
          <select value={valueType} onChange={(e) => setValueType(e.target.value)}>
            <option value="string">string</option>
            <option value="number">number</option>
            <option value="boolean">boolean</option>
            <option value="date">date</option>
            <option value="dropdown">dropdown</option>
          </select>
        </label>
        {needsOptions && (
          <label>
            Options
            <input
              value={enumValues}
              onChange={(e) => setEnumValues(e.target.value)}
              placeholder="au, nz, us"
              required
            />
          </label>
        )}
        <label className="perm">
          <input
            type="checkbox"
            checked={required}
            onChange={(e) => setRequired(e.target.checked)}
          />
          <span>Required</span>
        </label>
        <button type="submit">Add attribute</button>
      </form>

      {grouped.length === 0 && <p className="empty">No attributes defined yet.</p>}
      {grouped.map(([sec, items]) => (
        <fieldset key={sec} className="perm-group attr-section">
          <legend>{sec}</legend>
          <ul className="list">
            {items.map((d) => (
              <li key={d.id} className="member">
                <strong>{d.label}</strong>
                <span>{d.key}</span>
                <span>{d.value_type}</span>
                <span className="status">{d.required ? 'required' : 'optional'}</span>
              </li>
            ))}
          </ul>
        </fieldset>
      ))}
    </section>
  )
}
