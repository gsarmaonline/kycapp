import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  getAutomationCatalog,
  listEmailTemplates,
  type AutomationAction,
  type AutomationCatalog,
  type AutomationCondition,
  type AutomationConditionMode,
  type AutomationConditions,
} from '../../api'
import { AutomationDag } from './dag/AutomationDag'
import { normalizeActionWorkflow, normalizeGraph } from './dag/build_graph'

type Props = {
  submitLabel: string
  cancelTo: string
  initial?: {
    name: string
    trigger: string
    enabled: boolean
    conditionMode?: AutomationConditionMode
    conditions: AutomationCondition[]
    actions: AutomationAction[]
  }
  onSubmit: (input: {
    name: string
    trigger: string
    enabled: boolean
    conditions: AutomationConditions
    actions: AutomationAction[]
  }) => Promise<void>
}

function serializeConditionValue(
  op: string,
  value: AutomationCondition['value'],
  valueType?: string,
): AutomationCondition['value'] | undefined {
  if (op === 'exists' || op === 'not_exists') return undefined
  if (op === 'in' || op === 'not_in') {
    const list = Array.isArray(value)
      ? value.map((v) => String(v).trim()).filter(Boolean)
      : String(value ?? '')
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean)
    return list
  }
  if (valueType === 'boolean') {
    if (typeof value === 'boolean') return value
    const s = String(value ?? '').toLowerCase()
    return s === 'true' || s === '1'
  }
  if (valueType === 'number') {
    if (typeof value === 'number' && Number.isFinite(value)) return value
    const n = Number(String(value ?? '').trim())
    return Number.isFinite(n) ? n : String(value ?? '').trim()
  }
  return String(value ?? '').trim()
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
  const [conditionMode, setConditionMode] = useState<AutomationConditionMode>(
    initial?.conditionMode ?? 'all',
  )
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
      const fieldByKey = new Map((catalog?.condition_fields ?? []).map((f) => [f.field, f]))
      const cleanedConditions = conditions
        .map((c) => {
          const meta = fieldByKey.get(c.field)
          const value = serializeConditionValue(c.op, c.value, meta?.value_type)
          return {
            field: c.field.trim(),
            op: c.op,
            ...(value === undefined ? {} : { value }),
          }
        })
        .filter((c) => c.field)
      if (
        !cleanedConditions.length &&
        !trigger.startsWith('schedule.') &&
        trigger !== 'webhook.received'
      ) {
        throw new Error('Add at least one condition')
      }
      for (const c of cleanedConditions) {
        const opMeta = catalog?.ops.find((o) => o.op === c.op)
        if (opMeta?.needs_list) {
          const list = Array.isArray(c.value) ? c.value : []
          if (!list.length) {
            throw new Error(`Condition on ${c.field} needs at least one value for “${opMeta.label}”`)
          }
        } else if (opMeta?.needs_value && (c.value === undefined || c.value === '')) {
          throw new Error(`Condition on ${c.field} needs a value`)
        }
      }
      const cleanedActions = normalizeActionWorkflow(actions)
        .map((a) => {
          const params: Record<string, string> = { ...(a.params ?? {}) }
          if (a.template_key && !params.template_key) {
            params.template_key = a.template_key
          }
          for (const [k, v] of Object.entries(params)) {
            params[k] = String(v ?? '').trim()
          }
          const out: AutomationAction = {
            id: a.id,
            type: a.type.trim(),
            params,
          }
          if (a.on_success) out.on_success = a.on_success
          if (a.on_error) out.on_error = a.on_error
          return out
        })
        .filter((a) => a.type)
      if (!cleanedActions.length) {
        throw new Error('Add at least one action')
      }
      for (const a of cleanedActions) {
        if (a.on_success && !cleanedActions.some((x) => x.id === a.on_success)) {
          throw new Error(`Action ${a.id}: on_success target ${a.on_success} is missing`)
        }
        if (a.on_error && !cleanedActions.some((x) => x.id === a.on_error)) {
          throw new Error(`Action ${a.id}: on_error target ${a.on_error} is missing`)
        }
        if (a.type === 'delay' && !a.params?.duration) {
          throw new Error('Each delay action needs a duration (e.g. 5m, 1h)')
        }
        if (a.type === 'send_email' && !a.params?.template_key) {
          throw new Error('Each send_email action needs a template_key')
        }
        if (a.type === 'call_webhook' && !a.params?.webhook_id) {
          throw new Error('Each call_webhook action needs a webhook')
        }
        if (a.type === 'db_insert') {
          if (!a.params?.database_id) throw new Error('Each db_insert action needs a database')
          if (!a.params?.table) throw new Error('Each db_insert action needs a table')
          if (a.params.mode === 'columns') {
            try {
              const map = a.params.mapping ? JSON.parse(a.params.mapping) : {}
              if (!map || typeof map !== 'object' || !Object.keys(map).length) {
                throw new Error('Column mapping needs at least one column')
              }
            } catch (err) {
              if (err instanceof Error && err.message.includes('Column mapping')) throw err
              throw new Error('Column mapping must be valid JSON')
            }
          }
        }
      }
      await onSubmit({
        name: trimmedName,
        trigger,
        enabled,
        conditions: { mode: conditionMode, items: cleanedConditions },
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
        conditionMode={conditionMode}
        conditions={conditions}
        actions={actions}
        onTriggerChange={setTrigger}
        onConditionModeChange={setConditionMode}
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
