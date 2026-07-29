import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getProductFeature, type ProductFeature } from '../../api'
import { EntitlementUsageChart } from '../../components/EntitlementUsageChart'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function ProductFeaturesShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<ProductFeature | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getProductFeature(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
  }, [id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title={item.key} />
      <p className="row" style={{ gap: '0.75rem' }}>
        <Link className="button ghost" to={resourcePath(orgId, 'product-features', item.id, 'edit')}>
          Edit
        </Link>
        <Link className="button ghost" to={resourcePath(orgId, 'product-features')}>
          Back
        </Link>
      </p>
      <DetailList
        items={[
          { label: 'Key', value: item.key },
          { label: 'Description', value: item.description || '—' },
          { label: 'Scope', value: item.scope },
        ]}
      />
      <div className="obs-show-block">
        <EntitlementUsageChart
          orgId={orgId}
          entitlementKey={item.key}
          days={14}
          title={`Checks for ${item.key}`}
        />
      </div>
    </section>
  )
}
