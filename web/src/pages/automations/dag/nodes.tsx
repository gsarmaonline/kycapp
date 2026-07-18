import { Handle, Position, type Node, type NodeProps } from '@xyflow/react'
import type { AutomationCondition } from '../../../api'
import type { ActionNodeData, ConditionNodeData, TriggerNodeData } from './types'

export type TriggerFlowNode = Node<TriggerNodeData, 'trigger'>
export type ConditionFlowNode = Node<ConditionNodeData, 'condition'>
export type ActionFlowNode = Node<ActionNodeData, 'action'>
export type AutomationFlowNode = TriggerFlowNode | ConditionFlowNode | ActionFlowNode

export function TriggerNode({ data }: NodeProps<TriggerFlowNode>) {
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
          <option value="app_user.created">app_user.created</option>
          <option value="app_user.updated">app_user.updated</option>
        </select>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

export function ConditionNode({ data }: NodeProps<ConditionFlowNode>) {
  const c = data.condition
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
          <input
            value={c.field}
            onChange={(e) => data.onChange?.({ ...c, field: e.target.value })}
            placeholder="field"
          />
          <select
            value={c.op}
            onChange={(e) =>
              data.onChange?.({
                ...c,
                op: e.target.value as AutomationCondition['op'],
              })
            }
          >
            <option value="eq">eq</option>
            <option value="neq">neq</option>
            <option value="exists">exists</option>
            <option value="not_exists">not_exists</option>
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
          {a.template_key ? `: ${a.template_key}` : ''}
        </strong>
      ) : (
        <div className="dag-node-fields">
          <select
            value={a.type}
            onChange={(e) => data.onChange?.({ ...a, type: e.target.value })}
          >
            <option value="send_email">send_email</option>
          </select>
          <input
            value={a.template_key ?? ''}
            onChange={(e) => data.onChange?.({ ...a, template_key: e.target.value })}
            placeholder="template_key"
          />
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
