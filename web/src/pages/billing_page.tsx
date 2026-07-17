import { useParams } from 'react-router-dom'
import { BillingPanel } from '../panels/billing_panel'
import { useState } from 'react'

export function BillingPage() {
  const { orgId = '' } = useParams()
  const [error, setError] = useState<string | null>(null)
  return (
    <section>
      <h2 className="section-title">Billing</h2>
      {error && <p className="error">{error}</p>}
      <BillingPanel orgId={orgId} onError={setError} />
    </section>
  )
}
