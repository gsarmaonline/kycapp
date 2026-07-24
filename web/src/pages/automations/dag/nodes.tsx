import { Handle, Position, type Node, type NodeProps } from '@xyflow/react'
import type { AutomationAction, AutomationCatalog, AutomationCondition } from '../../../api'
import type { ActionNodeData, ConditionNodeData, TriggerNodeData } from './types'

export type TriggerFlowNode = Node<TriggerNodeData, 'trigger'>
export type ConditionFlowNode = Node<ConditionNodeData, 'condition'>
export type ActionFlowNode = Node<ActionNodeData, 'action'>
export type AutomationFlowNode = TriggerFlowNode | ConditionFlowNode | ActionFlowNode

function opsForField(
  field: AutomationCatalog['condition_fields'][number] | undefined,
  allOps: AutomationCatalog['ops'],
): AutomationCatalog['ops'] {
  if (field?.allowed_ops?.length) {
    const allowed = new Set(field.allowed_ops)
    const filtered = allOps.filter((o) => allowed.has(o.op))
    if (filtered.length) return filtered
  }
  if (field?.value_type) {
    const filtered = allOps.filter(
      (o) => !o.value_types?.length || o.value_types.includes(field.value_type),
    )
    if (filtered.length) return filtered
  }
  return allOps
}

function displayValue(value: AutomationCondition['value']): string {
  if (Array.isArray(value)) return value.join(', ')
  if (value == null) return ''
  return String(value)
}

function ConditionValueInput({
  condition,
  field,
  op,
  onChange,
}: {
  condition: AutomationCondition
  field?: AutomationCatalog['condition_fields'][number]
  op?: AutomationCatalog['ops'][number]
  onChange: (next: AutomationCondition) => void
}) {
  if (!op?.needs_value) return null

  const enums = field?.enum_values ?? []
  const valueType = field?.value_type ?? 'string'
  const isList = Boolean(op.needs_list)

  if (valueType === 'boolean' && !isList) {
    return (
      <select
        value={displayValue(condition.value) || 'true'}
        onChange={(e) => onChange({ ...condition, value: e.target.value })}
      >
        <option value="true">true</option>
        <option value="false">false</option>
      </select>
    )
  }

  if (enums.length > 0 && !isList) {
    return (
      <select
        value={displayValue(condition.value)}
        onChange={(e) => onChange({ ...condition, value: e.target.value })}
      >
        <option value="">Select…</option>
        {enums.map((v) => (
          <option key={v} value={v}>
            {v}
          </option>
        ))}
      </select>
    )
  }

  if (enums.length > 0 && isList) {
    return (
      <input
        value={displayValue(condition.value)}
        onChange={(e) => onChange({ ...condition, value: e.target.value })}
        placeholder={enums.slice(0, 3).join(', ') + (enums.length > 3 ? ', …' : '')}
        title="Comma-separated values"
      />
    )
  }

  if (valueType === 'date' && !isList) {
    return (
      <input
        type="date"
        value={displayValue(condition.value)}
        onChange={(e) => onChange({ ...condition, value: e.target.value })}
      />
    )
  }

  if (valueType === 'number' && !isList) {
    return (
      <input
        type="number"
        value={displayValue(condition.value)}
        onChange={(e) => onChange({ ...condition, value: e.target.value })}
        placeholder="number"
      />
    )
  }

  return (
    <input
      value={displayValue(condition.value)}
      onChange={(e) => onChange({ ...condition, value: e.target.value })}
      placeholder={isList ? 'a, b, c' : 'value'}
      title={isList ? 'Comma-separated values' : undefined}
    />
  )
}

export function TriggerNode({ data }: NodeProps<TriggerFlowNode>) {
  const triggers = data.triggers?.length
    ? data.triggers
    : [
        { id: 'app_user.created', label: 'App user created', description: '' },
        { id: 'app_user.updated', label: 'App user updated', description: '' },
      ]
  return (
    <div className={`dag-node dag-node-trigger${data.readOnly ? ' is-readonly' : ''}`}>
      <div className="dag-node-kind">Trigger</div>
      {data.readOnly ? (
        <strong className="dag-node-title">{data.trigger}</strong>
      ) : (
        <select
          className="dag-node-select"
          value={data.trigger}
          onChange={(e) => data.onTriggerChange?.(e.target.value)}
        >
          {triggers.map((t) => (
            <option key={t.id} value={t.id}>
              {t.label}
            </option>
          ))}
        </select>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

export function ConditionNode({ data }: NodeProps<ConditionFlowNode>) {
  const c = data.condition
  const fields = data.conditionFields ?? []
  const allOps = data.ops?.length
    ? data.ops
    : [
        { op: 'eq', label: 'equals', needs_value: true },
        { op: 'neq', label: 'not equals', needs_value: true },
        { op: 'exists', label: 'exists', needs_value: false },
        { op: 'not_exists', label: 'does not exist', needs_value: false },
      ]
  const fieldOptions = fields.length
    ? fields
    : [{ field: c.field || 'app_user.status', label: c.field || 'app_user.status', value_type: 'string', group: 'user' }]
  const knownField = fieldOptions.some((f) => f.field === c.field)
  const selectedField = fieldOptions.find((f) => f.field === c.field)
  let ops = opsForField(selectedField, allOps)
  if (c.op && !ops.some((o) => o.op === c.op)) {
    const orphan = allOps.find((o) => o.op === c.op)
    ops = orphan
      ? [orphan, ...ops]
      : [{ op: c.op, label: c.op, needs_value: true }, ...ops]
  }
  const selectedOp = ops.find((o) => o.op === c.op) ?? ops[0]

  return (
    <div className={`dag-node dag-node-condition${data.readOnly ? ' is-readonly' : ''}`}>
      <Handle type="target" position={Position.Left} />
      <div className="dag-node-kind">
        Condition
        {!data.readOnly && data.canRemove && (
          <button type="button" className="dag-node-remove" onClick={() => data.onRemove?.()}>
            ×
          </button>
        )}
      </div>
      {data.readOnly ? (
        <strong className="dag-node-title">
          {c.field} {c.op} {displayValue(c.value)}
        </strong>
      ) : (
        <div className="dag-node-fields">
          <select
            value={knownField ? c.field : c.field}
            onChange={(e) => {
              const nextField = e.target.value
              const meta = fieldOptions.find((f) => f.field === nextField)
              const nextOps = opsForField(meta, allOps)
              const nextOp = nextOps.some((o) => o.op === c.op) ? c.op : nextOps[0]?.op ?? 'eq'
              data.onChange?.({ ...c, field: nextField, op: nextOp, value: c.value ?? '' })
            }}
          >
            {!knownField && c.field && (
              <option value={c.field}>{c.field} (unavailable)</option>
            )}
            {fieldOptions.map((f) => (
              <option key={f.field} value={f.field}>
                {f.group === 'attributes' ? `Attribute · ${f.label}` : f.label}
              </option>
            ))}
          </select>
          <select
            value={selectedOp?.op ?? c.op}
            onChange={(e) =>
              data.onChange?.({
                ...c,
                op: e.target.value,
              })
            }
          >
            {ops.map((o) => (
              <option key={o.op} value={o.op}>
                {o.label}
              </option>
            ))}
          </select>
          <ConditionValueInput
            condition={c}
            field={selectedField}
            op={selectedOp}
            onChange={(next) => data.onChange?.(next)}
          />
        </div>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

function parseMappingRows(raw: string | undefined): { column: string; path: string }[] {
  if (!raw?.trim()) return [{ column: '', path: '' }]
  try {
    const obj = JSON.parse(raw) as Record<string, string>
    const rows = Object.entries(obj).map(([column, path]) => ({ column, path: String(path) }))
    return rows.length ? rows : [{ column: '', path: '' }]
  } catch {
    return [{ column: '', path: '' }]
  }
}

function serializeMappingRows(rows: { column: string; path: string }[]): string {
  const out: Record<string, string> = {}
  for (const r of rows) {
    const col = r.column.trim()
    const path = r.path.trim()
    if (col && path) out[col] = path
  }
  return Object.keys(out).length ? JSON.stringify(out) : ''
}

function DBInsertFields({
  params,
  databases,
  defaultDatabaseId,
  onChange,
}: {
  params: Record<string, string>
  databases: { id: string; name: string }[]
  defaultDatabaseId: string
  onChange: (params: Record<string, string>) => void
}) {
  const mode = params.mode === 'columns' ? 'columns' : 'event'
  const rows = parseMappingRows(params.mapping)

  function setParams(next: Record<string, string>) {
    onChange(next)
  }

  return (
    <>
      <select
        value={params.database_id || defaultDatabaseId}
        onChange={(e) => setParams({ ...params, database_id: e.target.value })}
        required
      >
        {!databases.length && <option value="">No databases</option>}
        {params.database_id &&
          !databases.some((d) => d.id === params.database_id) && (
            <option value={params.database_id}>{params.database_id} (missing)</option>
          )}
        {databases.map((d) => (
          <option key={d.id} value={d.id}>
            {d.name}
          </option>
        ))}
      </select>
      <input
        value={params.table ?? ''}
        onChange={(e) => setParams({ ...params, table: e.target.value })}
        placeholder="table (e.g. kyc_events)"
        required
      />
      <select
        value={mode}
        onChange={(e) => {
          const nextMode = e.target.value === 'columns' ? 'columns' : 'event'
          setParams({
            ...params,
            mode: nextMode,
            mapping: nextMode === 'event' ? '' : params.mapping || '{"email":"app_user.email"}',
          })
        }}
      >
        <option value="event">Event dump (trigger + payload jsonb)</option>
        <option value="columns">Column mapping</option>
      </select>
      {mode === 'columns' && (
        <div className="dag-map-rows">
          {rows.map((row, i) => (
            <div key={i} className="dag-map-row">
              <input
                value={row.column}
                placeholder="column"
                onChange={(e) => {
                  const next = rows.map((r, j) =>
                    j === i ? { ...r, column: e.target.value } : r,
                  )
                  setParams({ ...params, mode: 'columns', mapping: serializeMappingRows(next) })
                }}
              />
              <input
                value={row.path}
                placeholder="app_user.email"
                onChange={(e) => {
                  const next = rows.map((r, j) => (j === i ? { ...r, path: e.target.value } : r))
                  setParams({ ...params, mode: 'columns', mapping: serializeMappingRows(next) })
                }}
              />
              <button
                type="button"
                className="dag-node-remove"
                title="Remove row"
                onClick={() => {
                  const next = rows.filter((_, j) => j !== i)
                  setParams({
                    ...params,
                    mode: 'columns',
                    mapping: serializeMappingRows(next.length ? next : [{ column: '', path: '' }]),
                  })
                }}
              >
                ×
              </button>
            </div>
          ))}
          <button
            type="button"
            className="ghost"
            onClick={() =>
              setParams({
                ...params,
                mode: 'columns',
                mapping: serializeMappingRows([...rows, { column: '', path: '' }]),
              })
            }
          >
            Add column
          </button>
        </div>
      )}
    </>
  )
}

export function ActionNode({ data }: NodeProps<ActionFlowNode>) {
  const a = data.action
  const actions = data.actions?.length
    ? data.actions
    : [{ type: 'send_email', label: 'Send email', description: '', params: [] }]
  const selected = actions.find((act) => act.type === a.type) ?? actions[0]
  const params = { ...(a.params ?? {}) }
  if (a.template_key && !params.template_key) {
    params.template_key = a.template_key
  }
  const paramDefs = selected?.params?.length
    ? selected.params
    : Object.keys(params).map((key) => ({ key, label: key, required: false }))
  const templates = data.emailTemplates ?? []
  const databases = data.databases ?? []
  const webhooks = data.webhooks ?? []
  const defaultTemplateKey = templates[0]?.key ?? ''
  const defaultDatabaseId = databases[0]?.id ?? ''
  const defaultWebhookId = webhooks[0]?.id ?? ''
  const siblings = (data.siblingActions ?? []).filter((s) => s.id && s.id !== a.id)

  function patch(next: Partial<AutomationAction>) {
    data.onChange?.({
      id: a.id,
      type: next.type ?? a.type,
      params: next.params ?? params,
      on_success: next.on_success !== undefined ? next.on_success || undefined : a.on_success,
      on_error: next.on_error !== undefined ? next.on_error || undefined : a.on_error,
      template_key: a.template_key,
    })
  }

  const summary = (() => {
    if (a.type === 'db_insert') {
      const db = databases.find((x) => x.id === params.database_id)
      const mode = params.mode === 'columns' ? 'columns' : 'event'
      return [db?.name || params.database_id, params.table, mode].filter(Boolean).join(', ')
    }
    return paramDefs
      .map((p) => {
        if (p.key === 'template_key') {
          const t = templates.find((x) => x.key === params[p.key])
          return t ? t.name : params[p.key]
        }
        if (p.key === 'database_id') {
          const d = databases.find((x) => x.id === params[p.key])
          return d ? d.name : params[p.key]
        }
        if (p.key === 'webhook_id') {
          const w = webhooks.find((x) => x.id === params[p.key])
          return w ? w.name : params[p.key]
        }
        if (p.key === 'mapping' || p.key === 'mode') return ''
        return params[p.key]
      })
      .filter(Boolean)
      .join(', ')
  })()

  return (
    <div className={`dag-node dag-node-action${data.readOnly ? ' is-readonly' : ''}`}>
      <Handle type="target" position={Position.Left} />
      <div className="dag-node-kind">
        Action{a.id ? ` · ${a.id}` : ''}
        {!data.readOnly && data.canRemove && (
          <button type="button" className="dag-node-remove" onClick={() => data.onRemove?.()}>
            ×
          </button>
        )}
      </div>
      {data.readOnly ? (
        <strong className="dag-node-title">
          {a.type}
          {summary ? `: ${summary}` : ''}
          {a.on_error ? ` (on error → ${a.on_error})` : ''}
        </strong>
      ) : (
        <div className="dag-node-fields">
          <select
            value={a.type}
            onChange={(e) => {
              const nextType = e.target.value
              const nextInfo = actions.find((act) => act.type === nextType)
              const nextParams: Record<string, string> = {}
              for (const p of nextInfo?.params ?? []) {
                if (p.key === 'template_key') {
                  nextParams[p.key] = params[p.key] || defaultTemplateKey
                } else if (p.key === 'database_id') {
                  nextParams[p.key] = params[p.key] || defaultDatabaseId
                } else if (p.key === 'webhook_id') {
                  nextParams[p.key] = params[p.key] || defaultWebhookId
                } else if (p.key === 'mode') {
                  nextParams[p.key] = 'event'
                } else if (p.key === 'mapping') {
                  nextParams[p.key] = ''
                } else {
                  nextParams[p.key] = params[p.key] ?? ''
                }
              }
              patch({ type: nextType, params: nextParams })
            }}
          >
            {actions.map((act) => (
              <option key={act.type} value={act.type}>
                {act.label}
              </option>
            ))}
          </select>
          {a.type === 'db_insert' ? (
            <DBInsertFields
              params={params}
              databases={databases}
              defaultDatabaseId={defaultDatabaseId}
              onChange={(next) => patch({ params: next })}
            />
          ) : (
            paramDefs.map((p) => {
              if (p.key === 'template_key') {
                return (
                  <select
                    key={p.key}
                    value={params[p.key] || defaultTemplateKey}
                    onChange={(e) => patch({ params: { ...params, [p.key]: e.target.value } })}
                    required={p.required}
                  >
                    {!templates.length && <option value="">No templates</option>}
                    {params[p.key] &&
                      !templates.some((t) => t.key === params[p.key]) && (
                        <option value={params[p.key]}>{params[p.key]} (missing)</option>
                      )}
                    {templates.map((t) => (
                      <option key={t.key} value={t.key}>
                        {t.name}
                      </option>
                    ))}
                  </select>
                )
              }
              if (p.key === 'webhook_id') {
                return (
                  <select
                    key={p.key}
                    value={params[p.key] || defaultWebhookId}
                    onChange={(e) => patch({ params: { ...params, [p.key]: e.target.value } })}
                    required={p.required}
                  >
                    {!webhooks.length && <option value="">No webhooks</option>}
                    {params[p.key] &&
                      !webhooks.some((w) => w.id === params[p.key]) && (
                        <option value={params[p.key]}>{params[p.key]} (missing)</option>
                      )}
                    {webhooks.map((w) => (
                      <option key={w.id} value={w.id}>
                        {w.name}
                      </option>
                    ))}
                  </select>
                )
              }
              if (p.key === 'database_id' || p.key === 'mapping' || p.key === 'mode') {
                return null
              }
              return (
                <input
                  key={p.key}
                  type={p.key === 'secret' || p.key === 'password' ? 'password' : 'text'}
                  value={params[p.key] ?? ''}
                  onChange={(e) => patch({ params: { ...params, [p.key]: e.target.value } })}
                  placeholder={p.label || p.key}
                />
              )
            })
          )}
          <label className="dag-edge-field">
            On error
            <select
              value={a.on_error || ''}
              onChange={(e) => patch({ on_error: e.target.value })}
            >
              <option value="">Fail the run</option>
              {siblings.map((s) => (
                <option key={s.id} value={s.id}>
                  → {s.id} ({s.type})
                </option>
              ))}
            </select>
          </label>
        </div>
      )}
      <Handle type="source" position={Position.Right} id="success" />
      <Handle
        type="source"
        position={Position.Bottom}
        id="error"
        className="dag-handle-error"
      />
    </div>
  )
}

export const automationNodeTypes = {
  trigger: TriggerNode,
  condition: ConditionNode,
  action: ActionNode,
}
