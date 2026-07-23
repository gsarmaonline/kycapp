import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { createProductPlan } from '../../api'
import { FormActions, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

export function ProductPlansNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [withPrice, setWithPrice] = useState(false)
  const [interval, setInterval] = useState('month')
  const [currency, setCurrency] = useState('usd')
  const [amount, setAmount] = useState('29.00')
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const cents = Math.round(parseFloat(amount || '0') * 100)
      if (withPrice && (Number.isNaN(cents) || cents < 0)) {
        setError('Enter a valid price amount')
        return
      }
      const p = await createProductPlan(orgId, {
        key,
        name,
        ...(withPrice
          ? { price: { interval, currency, unit_amount: cents } }
          : {}),
      })
      navigate(resourcePath(orgId, 'product-plans', p.id, 'edit'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  return (
    <section>
      <PageHeader title="Create product plan" />
      {error && <p className="error">{error}</p>}
      <form className="create stacked" onSubmit={onSubmit}>
        <label>
          Key
          <input value={key} onChange={(e) => setKey(e.target.value)} placeholder="pro" required />
        </label>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Pro" required />
        </label>
        <label className="perm">
          <input
            type="checkbox"
            checked={withPrice}
            onChange={(e) => setWithPrice(e.target.checked)}
          />
          <span>Add recurring price (syncs to Stripe when connected)</span>
        </label>
        {withPrice && (
          <>
            <label>
              Interval
              <select value={interval} onChange={(e) => setInterval(e.target.value)}>
                <option value="month">month</option>
                <option value="year">year</option>
              </select>
            </label>
            <label>
              Currency
              <input value={currency} onChange={(e) => setCurrency(e.target.value)} required />
            </label>
            <label>
              Amount
              <input
                type="number"
                min="0"
                step="0.01"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                required
              />
            </label>
          </>
        )}
        <FormActions cancelTo={resourcePath(orgId, 'product-plans')} submitLabel="Create" />
      </form>
    </section>
  )
}
