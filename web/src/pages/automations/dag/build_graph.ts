import type { Edge } from '@xyflow/react'
import type { AutomationAction, AutomationCondition } from '../../../api'
import type { AutomationFlowNode } from './nodes'

const X_TRIGGER = 40
const X_CONDITION = 300
const X_ACTION = 620
const Y_STEP = 150

const defaultCondition = (): AutomationCondition => ({
  field: 'attributes.country',
  op: 'eq',
  value: '',
})

const defaultAction = (): AutomationAction => ({
  type: 'send_email',
  template_key: 'welcome',
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
        canRemove: graph.actions.length > 1,
        onChange: (next) => opts.onActionChange?.(i, next),
        onRemove: () => opts.onActionRemove?.(i),
      },
      draggable: false,
      selectable: !opts.readOnly,
    })
  })

  const edges: Edge[] = []
  if (graph.conditions.length === 0) {
    if (graph.actions.length > 0) {
      edges.push({
        id: 'e-trigger-action-0',
        source: 'trigger',
        target: 'action-0',
        animated: true,
      })
    }
  } else {
    graph.conditions.forEach((_, i) => {
      edges.push({
        id: `e-trigger-cond-${i}`,
        source: 'trigger',
        target: `cond-${i}`,
        animated: true,
      })
      if (graph.actions.length > 0) {
        edges.push({
          id: `e-cond-${i}-action-0`,
          source: `cond-${i}`,
          target: 'action-0',
        })
      }
    })
  }

  for (let i = 0; i < graph.actions.length - 1; i++) {
    edges.push({
      id: `e-action-${i}-${i + 1}`,
      source: `action-${i}`,
      target: `action-${i + 1}`,
      animated: true,
    })
  }

  return { nodes, edges }
}

export { defaultAction, defaultCondition }
