import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import type { AutomationAction, AutomationCondition } from '../../api'

type Props = {
  submitLabel: string
  cancelTo: string
  initial?: {
    name: string
    trigger: string
    enabled: boolean
    conditions: AutomationCondition[]
    actions: AutomationAction[]
  }
  onSubmit: (input: {
    name: string
    trigger: string
    enabled: boolean
    conditions: { all: AutomationCondition[] }
    actions: AutomationAction[]
  }) => Promise<void>
}

const emptyCondition = (): AutomationCondition => ({
  field: 'attributes.country',
  op: 'eq',
  value: '',
})

export function AutomationsForm({ submitLabel, cancelTo, initial, onSubmit }: Props) {
  const [name, setName] = useState(initial?.name ?? '')
  const [trigger, setTrigger] = useState(initial?.trigger ?? 'app_user.created')
  const [enabled, setEnabled] = useState(initial?.enabled ?? true)
  const [conditions, setConditions] = useState<AutomationCondition[]>(
    initial?.conditions?.length ? initial.conditions : [emptyCondition()],
  )
  const [templateKey, setTemplateKey] = useState(
    initial?.actions?.[0]?.template_key ?? 'welcome',
  )
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await onSubmit({
        name,
        trigger,
        enabled,
        conditions: {
          all: conditions
            .map((c) => ({
              field: c.field.trim(),
              op: c.op,
              value: c.op === 'exists' || c.op === 'not_exists' ? undefined : c.value,
            }))
            .filter((c) => c.field),
        },
        actions: [{ type: 'send_email', template_key: templateKey.trim() }],
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <form className="create stacked" onSubmit={(e) => void handleSubmit(e)}>
      {error && <p className="error">{error}</p>}
      <label>
        Name
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Optional" />
      </label>
      <label>
        Trigger
        <select value={trigger} onChange={(e) => setTrigger(e.target.value)}>
          <option value="app_user.created">app_user.created</option>
          <option value="app_user.updated">app_user.updated</option>
        </select>
      </label>
      <label className="perm">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => setEnabled(e.target.checked)}
        />
        <span>Enabled</span>
      </label>

      <fieldset className="stacked">
        <legend>Conditions (all must match)</legend>
        {conditions.map((c, i) => (
          <div key={i} className="condition-row">
            <input
              value={c.field}
              onChange={(e) => {
                const next = [...conditions]
                next[i] = { ...c, field: e.target.value }
                setConditions(next)
              }}
              placeholder="field (e.g. attributes.country)"
              required
            />
            <select
              value={c.op}
              onChange={(e) => {
                const next = [...conditions]
                next[i] = { ...c, op: e.target.value as AutomationCondition['op'] }
                setConditions(next)
              }}
            >
              <option value="eq">eq</option>
              <option value="neq">neq</option>
              <option value="exists">exists</option>
              <option value="not_exists">not_exists</option>
            </select>
            {c.op !== 'exists' && c.op !== 'not_exists' && (
              <input
                value={String(c.value ?? '')}
                onChange={(e) => {
                  const next = [...conditions]
                  next[i] = { ...c, value: e.target.value }
                  setConditions(next)
                }}
                placeholder="value"
                required
              />
            )}
            <button
              type="button"
              className="ghost"
              onClick={() => setConditions(conditions.filter((_, j) => j !== i))}
              disabled={conditions.length <= 1}
            >
              Remove
            </button>
          </div>
        ))}
        <button type="button" className="ghost" onClick={() => setConditions([...conditions, emptyCondition()])}>
          Add condition
        </button>
      </fieldset>

      <label>
        Action: send email template
        <input
          value={templateKey}
          onChange={(e) => setTemplateKey(e.target.value)}
          placeholder="welcome"
          required
        />
      </label>

      <div className="form-actions">
        <Link className="ghost" to={cancelTo}>
          Cancel
        </Link>
        <button type="submit">{submitLabel}</button>
      </div>
    </form>
  )
}
