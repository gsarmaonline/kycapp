import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createAttributeDefinition } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function AttributesNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [label, setLabel] = useState('')
  const [section, setSection] = useState('general')
  const [valueType, setValueType] = useState('string')
  const [required, setRequired] = useState(false)
  const [enumValues, setEnumValues] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const d = await createAttributeDefinition(orgId, {
        key,
        label,
        section: section || 'general',
        value_type: valueType,
        required,
        enum_values:
          valueType === 'dropdown'
            ? enumValues
                .split(',')
                .map((v) => v.trim())
                .filter(Boolean)
            : undefined,
      })
      navigate(resourcePath(orgId, 'attributes', d.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Create attribute" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input value={key} onChange={(e) => setKey(e.target.value)} required />
        </label>
        <label>
          Label
          <input value={label} onChange={(e) => setLabel(e.target.value)} required />
        </label>
        <label>
          Section
          <input value={section} onChange={(e) => setSection(e.target.value)} />
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
        {valueType === 'dropdown' && (
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
        <FormActions cancelTo={resourcePath(orgId, 'attributes')} submitLabel="Create" />
      </form>
    </section>
  )
}
