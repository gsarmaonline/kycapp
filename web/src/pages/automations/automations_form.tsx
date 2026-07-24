import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  getAutomationCatalog,
  listEmailTemplates,
  type AutomationAction,
  type AutomationCatalog,
  type AutomationCondition,
} from '../../api'
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
  const { orgId = '' } = useParams()
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
  const [catalog, setCatalog] = useState<AutomationCatalog | null>(null)
  const [emailTemplates, setEmailTemplates] = useState<{ key: string; name: string }[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!orgId) return
    void Promise.all([getAutomationCatalog(orgId), listEmailTemplates(orgId, 'active')])
      .then(([c, templates]) => {
        setCatalog(c)
        const items = templates.items.map((t) => ({ key: t.key, name: t.name || t.key }))
        setEmailTemplates(items)
        if (!initial?.trigger && c.triggers[0]) {
          setTrigger(c.triggers[0].id)
        }
        if (!initial?.actions?.length && items[0]) {
          setActions((prev) =>
            prev.map((a) =>
              a.type === 'send_email' && !a.params?.template_key && !a.template_key
                ? { ...a, params: { ...a.params, template_key: items[0].key } }
                : a,
            ),
          )
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load automation catalog')
      })
  }, [orgId, initial?.trigger, initial?.actions?.length])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const trimmedName = name.trim()
      if (!trimmedName) {
        throw new Error('Name is required')
      }
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
        .map((a) => {
          const params: Record<string, string> = { ...(a.params ?? {}) }
          if (a.template_key && !params.template_key) {
            params.template_key = a.template_key
          }
          for (const [k, v] of Object.entries(params)) {
            params[k] = String(v ?? '').trim()
          }
          return {
            type: a.type.trim(),
            params,
          }
        })
        .filter((a) => a.type)
      if (!cleanedActions.length) {
        throw new Error('Add at least one action')
      }
      for (const a of cleanedActions) {
        if (a.type === 'send_email' && !a.params.template_key) {
          throw new Error('Each send_email action needs a template_key')
        }
      }
      await onSubmit({
        name: trimmedName,
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
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Welcome AU users"
            required
          />
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
        catalog={catalog}
        emailTemplates={emailTemplates}
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
