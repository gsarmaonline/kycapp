import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getAttributeDefinition, type AttributeDefinition } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function AttributesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<AttributeDefinition | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAttributeDefinition(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Attribute" />
      <DetailList
        items={[
          { label: 'Label', value: item.label },
          { label: 'Key', value: item.key },
          { label: 'Type', value: item.value_type },
          { label: 'Section', value: item.section },
          { label: 'Required', value: item.required ? 'yes' : 'no' },
          { label: 'PII', value: item.is_pii ? 'yes' : 'no' },
          { label: 'Status', value: item.status },
          {
            label: 'Options',
            value: item.enum_values?.length ? item.enum_values.join(', ') : '—',
          },
        ]}
      />
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'attributes')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'attributes', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
