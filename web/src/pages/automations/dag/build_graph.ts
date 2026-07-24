import type { Edge } from '@xyflow/react'
import type { AutomationAction, AutomationCatalog, AutomationCondition } from '../../../api'
import type { AutomationFlowNode } from './nodes'

const X_TRIGGER = 40
const X_CONDITION = 300
const X_ACTION = 620
const Y_STEP = 150

const defaultCondition = (preferredField = 'status'): AutomationCondition => ({
  field: preferredField,
  op: 'eq',
  value: '',
})

const defaultAction = (
  preferredType = 'send_email',
  preferredTemplateKey = '',
): AutomationAction => ({
  type: preferredType,
  params:
    preferredType === 'send_email'
      ? { template_key: preferredTemplateKey }
      : {},
})

export function normalizeGraph(
  input: {
    trigger?: string
    conditions?: AutomationCondition[]
    actions?: AutomationAction[]
  },
  opts?: { ensureDefaults?: boolean },
) {
  const trigger = input.trigger || 'app_user.created'
  let conditions = input.conditions ? [...input.conditions] : []
  let actions = input.actions ? [...input.actions] : []
  if (opts?.ensureDefaults) {
    if (!conditions.length) conditions = [defaultCondition()]
    if (!actions.length) actions = [defaultAction()]
  }
  return { trigger, conditions, actions }
}

export function buildFlowElements(
  graph: {
    trigger: string
    conditions: AutomationCondition[]
    actions: AutomationAction[]
  },
  opts: {
    readOnly: boolean
    catalog?: AutomationCatalog | null
    emailTemplates?: { key: string; name: string }[]
    onTriggerChange?: (trigger: string) => void
    onConditionChange?: (index: number, condition: AutomationCondition) => void
    onConditionRemove?: (index: number) => void
    onActionChange?: (index: number, action: AutomationAction) => void
    onActionRemove?: (index: number) => void
  },
): { nodes: AutomationFlowNode[]; edges: Edge[] } {
  const condCount = Math.max(graph.conditions.length, 1)
  const actionCount = Math.max(graph.actions.length, 1)
  const rowCount = Math.max(condCount, actionCount)
  const midY = ((rowCount - 1) * Y_STEP) / 2

  const nodes: AutomationFlowNode[] = [
    {
      id: 'trigger',
      type: 'trigger',
      position: { x: X_TRIGGER, y: midY },
      data: {
        trigger: graph.trigger,
        readOnly: opts.readOnly,
        triggers: opts.catalog?.triggers,
        onTriggerChange: opts.onTriggerChange,
      },
      draggable: false,
      selectable: !opts.readOnly,
    },
  ]

  graph.conditions.forEach((condition, i) => {
    nodes.push({
      id: `cond-${i}`,
      type: 'condition',
      position: { x: X_CONDITION, y: i * Y_STEP },
      data: {
        condition,
        readOnly: opts.readOnly,
        conditionFields: opts.catalog?.condition_fields,
        ops: opts.catalog?.ops,
        canRemove: graph.conditions.length > 1,
        onChange: (next) => opts.onConditionChange?.(i, next),
        onRemove: () => opts.onConditionRemove?.(i),
      },
      draggable: false,
      selectable: !opts.readOnly,
    })
  })

  graph.actions.forEach((action, i) => {
    nodes.push({
      id: `action-${i}`,
      type: 'action',
      position: { x: X_ACTION, y: i * Y_STEP },
      data: {
        action,
        readOnly: opts.readOnly,
        actions: opts.catalog?.actions,
        emailTemplates: opts.emailTemplates,
        databases: opts.catalog?.databases,
        canRemove: graph.actions.length > 1,
        onChange: (next) => opts.onActionChange?.(i, next),
        onRemove: () => opts.onActionRemove?.(i),
      },
      draggable: false,
      selectable: !opts.readOnly,
    })
  })

  // Topology (fixed v1 semantics):
  //   trigger → every condition (AND)
  //   every condition → every action (all actions run when conditions match)
  // Actions still execute in list order at runtime; the graph shows fan-out, not
  // per-condition branching (that would need a richer DSL).
  const edges: Edge[] = []
  if (graph.conditions.length === 0) {
    graph.actions.forEach((_, j) => {
      edges.push({
        id: `e-trigger-action-${j}`,
        source: 'trigger',
        target: `action-${j}`,
        animated: true,
      })
    })
  } else {
    graph.conditions.forEach((_, i) => {
      edges.push({
        id: `e-trigger-cond-${i}`,
        source: 'trigger',
        target: `cond-${i}`,
        animated: true,
      })
      graph.actions.forEach((_, j) => {
        edges.push({
          id: `e-cond-${i}-action-${j}`,
          source: `cond-${i}`,
          target: `action-${j}`,
        })
      })
    })
  }

  return { nodes, edges }
}

export { defaultAction, defaultCondition }
