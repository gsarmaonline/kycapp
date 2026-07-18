import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getAttributeDefinition, updateAttributeDefinition } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function AttributesEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [label, setLabel] = useState('')
  const [section, setSection] = useState('general')
  const [valueType, setValueType] = useState('string')
  const [required, setRequired] = useState(false)
  const [status, setStatus] = useState('active')
  const [isSystem, setIsSystem] = useState(false)
  const [enumValues, setEnumValues] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void getAttributeDefinition(id)
      .then((d) => {
        setLabel(d.label)
        setSection(d.section)
        setValueType(d.value_type)
        setRequired(d.required)
        setStatus(d.status)
        setIsSystem(!!d.is_system)
        setEnumValues((d.enum_values || []).join(', '))
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [id])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateAttributeDefinition(id, {
        label,
        section,
        value_type: valueType,
        required,
        status,
        enum_values:
          valueType === 'dropdown'
            ? enumValues
                .split(',')
                .map((v) => v.trim())
                .filter(Boolean)
            : undefined,
      })
      navigate(resourcePath(orgId, 'attributes', id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit attribute" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
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
            <input value={enumValues} onChange={(e) => setEnumValues(e.target.value)} required />
          </label>
        )}
        <label>
          Status
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            disabled={isSystem}
            title={isSystem ? 'System attributes cannot be archived' : undefined}
          >
            <option value="active">active</option>
            {!isSystem && <option value="archived">archived</option>}
          </select>
        </label>
        <label className="perm">
          <input
            type="checkbox"
            checked={required}
            onChange={(e) => setRequired(e.target.checked)}
          />
          <span>Required</span>
        </label>
        <FormActions cancelTo={resourcePath(orgId, 'attributes', id)} submitLabel="Save" />
      </form>
    </section>
  )
}
