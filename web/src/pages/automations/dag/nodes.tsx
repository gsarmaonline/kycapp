import { Handle, Position, type Node, type NodeProps } from '@xyflow/react'
import type { AutomationCondition } from '../../../api'
import type { ActionNodeData, ConditionNodeData, TriggerNodeData } from './types'

export type TriggerFlowNode = Node<TriggerNodeData, 'trigger'>
export type ConditionFlowNode = Node<ConditionNodeData, 'condition'>
export type ActionFlowNode = Node<ActionNodeData, 'action'>
export type AutomationFlowNode = TriggerFlowNode | ConditionFlowNode | ActionFlowNode

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
  const ops = data.ops?.length
    ? data.ops
    : [
        { op: 'eq', label: 'equals', needs_value: true },
        { op: 'neq', label: 'not equals', needs_value: true },
        { op: 'exists', label: 'exists', needs_value: false },
        { op: 'not_exists', label: 'does not exist', needs_value: false },
      ]
  const fieldOptions = fields.length
    ? fields
    : [{ field: c.field || 'status', label: c.field || 'status', value_type: 'string', group: 'user' }]
  const knownField = fieldOptions.some((f) => f.field === c.field)

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
          {c.field} {c.op} {c.value ?? ''}
        </strong>
      ) : (
        <div className="dag-node-fields">
          <select
            value={knownField ? c.field : c.field}
            onChange={(e) => data.onChange?.({ ...c, field: e.target.value })}
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
            value={c.op}
            onChange={(e) =>
              data.onChange?.({
                ...c,
                op: e.target.value as AutomationCondition['op'],
              })
            }
          >
            {ops.map((o) => (
              <option key={o.op} value={o.op}>
                {o.label}
              </option>
            ))}
          </select>
          {c.op !== 'exists' && c.op !== 'not_exists' && (
            <input
              value={String(c.value ?? '')}
              onChange={(e) => data.onChange?.({ ...c, value: e.target.value })}
              placeholder="value"
            />
          )}
        </div>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
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

  const summary = paramDefs
    .map((p) => params[p.key])
    .filter(Boolean)
    .join(', ')

  return (
    <div className={`dag-node dag-node-action${data.readOnly ? ' is-readonly' : ''}`}>
      <Handle type="target" position={Position.Left} />
      <div className="dag-node-kind">
        Action
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
                nextParams[p.key] = params[p.key] ?? (p.key === 'template_key' ? 'welcome' : '')
              }
              data.onChange?.({ type: nextType, params: nextParams })
            }}
          >
            {actions.map((act) => (
              <option key={act.type} value={act.type}>
                {act.label}
              </option>
            ))}
          </select>
          {paramDefs.map((p) => (
            <input
              key={p.key}
              value={params[p.key] ?? ''}
              onChange={(e) =>
                data.onChange?.({
                  type: a.type,
                  params: { ...params, [p.key]: e.target.value },
                })
              }
              placeholder={p.label || p.key}
            />
          ))}
        </div>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

export const automationNodeTypes = {
  trigger: TriggerNode,
  condition: ConditionNode,
  action: ActionNode,
}
