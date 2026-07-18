import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import type { AutomationAction, AutomationCondition } from '../../api'
import { AutomationDag } from './dag/AutomationDag'
import { normalizeGraph } from './dag/build_graph'

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

export function AutomationsForm({ submitLabel, cancelTo, initial, onSubmit }: Props) {
  const initialGraph = normalizeGraph(
    {
      trigger: initial?.trigger,
      conditions: initial?.conditions,
      actions: initial?.actions,
    },
    { ensureDefaults: true },
  )
  const [name, setName] = useState(initial?.name ?? '')
  const [enabled, setEnabled] = useState(initial?.enabled ?? true)
  const [trigger, setTrigger] = useState(initialGraph.trigger)
  const [conditions, setConditions] = useState(initialGraph.conditions)
  const [actions, setActions] = useState(initialGraph.actions)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const cleanedConditions = conditions
        .map((c) => ({
          field: c.field.trim(),
          op: c.op,
          value: c.op === 'exists' || c.op === 'not_exists' ? undefined : String(c.value ?? '').trim(),
        }))
        .filter((c) => c.field)
      if (!cleanedConditions.length) {
        throw new Error('Add at least one condition')
      }
      const cleanedActions = actions
        .map((a) => ({
          type: a.type.trim(),
          template_key: (a.template_key ?? '').trim(),
        }))
        .filter((a) => a.type)
      if (!cleanedActions.length) {
        throw new Error('Add at least one action')
      }
      for (const a of cleanedActions) {
        if (a.type === 'send_email' && !a.template_key) {
          throw new Error('Each send_email action needs a template_key')
        }
      }
      await onSubmit({
        name,
        trigger,
        enabled,
        conditions: { all: cleanedConditions },
        actions: cleanedActions,
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <form className="automation-form" onSubmit={(e) => void handleSubmit(e)}>
      {error && <p className="error">{error}</p>}
      <div className="automation-meta">
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Optional" />
        </label>
        <label className="perm">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          <span>Enabled</span>
        </label>
      </div>

      <AutomationDag
        trigger={trigger}
        conditions={conditions}
        actions={actions}
        onTriggerChange={setTrigger}
        onConditionsChange={setConditions}
        onActionsChange={setActions}
      />

      <div className="form-actions">
        <Link className="ghost" to={cancelTo}>
          Cancel
        </Link>
        <button type="submit">{submitLabel}</button>
      </div>
    </form>
  )
}
