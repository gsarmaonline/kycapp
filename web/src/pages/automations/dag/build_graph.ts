import type { Edge } from '@xyflow/react'
import type { AutomationAction, AutomationCatalog, AutomationCondition } from '../../../api'
import type { AutomationFlowNode } from './nodes'

const X_TRIGGER = 40
const X_CONDITION = 280
const X_ACTION = 560
const Y_STEP = 170

const defaultCondition = (preferredField = 'app_user.status'): AutomationCondition => ({
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

/** Assign ids and linear on_success when no explicit edges exist (matches server). */
export function normalizeActionWorkflow(actions: AutomationAction[]): AutomationAction[] {
  if (!actions.length) return actions
  const used = new Map<string, number>()
  const withIds = actions.map((a, i) => {
    let id = (a.id || `a${i + 1}`).trim()
    const n = used.get(id) ?? 0
    if (n > 0) {
      id = `${id}_${n + 1}`
      used.set(a.id || `a${i + 1}`, n + 1)
    } else {
      used.set(id, 1)
    }
    return { ...a, id }
  })
  const hasEdges = withIds.some((a) => a.on_success || a.on_error)
  if (!hasEdges) {
    return withIds.map((a, i) => ({
      ...a,
      on_success: i < withIds.length - 1 ? withIds[i + 1].id : undefined,
      on_error: a.on_error || undefined,
    }))
  }
  return withIds
}

export function appendAction(
  actions: AutomationAction[],
  action: AutomationAction,
): AutomationAction[] {
  const base = normalizeActionWorkflow(actions)
  const id = `a${base.length + 1}`
  const next: AutomationAction = { ...action, id }
  if (!base.length) return [next]
  const last = base[base.length - 1]
  return [...base.slice(0, -1), { ...last, on_success: id }, next]
}

export function removeActionAt(
  actions: AutomationAction[],
  index: number,
): AutomationAction[] {
  if (actions.length <= 1) return actions
  const base = normalizeActionWorkflow(actions)
  const removed = base[index]
  const next = base.filter((_, i) => i !== index).map((a) => {
    const patch = { ...a }
    if (patch.on_success === removed.id) patch.on_success = removed.on_success
    if (patch.on_error === removed.id) patch.on_error = undefined
    return patch
  })
  return next
}

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
  actions = normalizeActionWorkflow(actions)
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
  const actions = normalizeActionWorkflow(graph.actions)
  const condCount = Math.max(graph.conditions.length, 1)
  const actionCount = Math.max(actions.length, 1)
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

  actions.forEach((action, i) => {
    nodes.push({
      id: `action-${action.id || i}`,
      type: 'action',
      position: { x: X_ACTION + (i % 2) * 40, y: i * Y_STEP },
      data: {
        action,
        actionIndex: i,
        siblingActions: actions,
        readOnly: opts.readOnly,
        actions: opts.catalog?.actions,
        emailTemplates: opts.emailTemplates,
        databases: opts.catalog?.databases,
        webhooks: opts.catalog?.webhooks,
        canRemove: actions.length > 1,
        onChange: (next) => opts.onActionChange?.(i, next),
        onRemove: () => opts.onActionRemove?.(i),
      },
      draggable: false,
      selectable: !opts.readOnly,
    })
  })

  const edges: Edge[] = []
  const entryId = actions[0] ? `action-${actions[0].id}` : undefined

  if (graph.conditions.length === 0) {
    if (entryId) {
      edges.push({
        id: 'e-trigger-entry',
        source: 'trigger',
        target: entryId,
        animated: true,
        label: 'then',
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
      if (entryId) {
        edges.push({
          id: `e-cond-${i}-entry`,
          source: `cond-${i}`,
          target: entryId,
          label: i === 0 ? 'then' : undefined,
        })
      }
    })
  }

  actions.forEach((a) => {
    const src = `action-${a.id}`
    if (a.on_success) {
      edges.push({
        id: `e-ok-${a.id}-${a.on_success}`,
        source: src,
        sourceHandle: 'success',
        target: `action-${a.on_success}`,
        label: 'ok',
        style: { stroke: 'var(--ok, #2f6f4e)' },
      })
    }
    if (a.on_error) {
      edges.push({
        id: `e-err-${a.id}-${a.on_error}`,
        source: src,
        sourceHandle: 'error',
        target: `action-${a.on_error}`,
        label: 'error',
        animated: true,
        style: { stroke: 'var(--danger, #a33)' },
      })
    }
  })

  return { nodes, edges }
}

export { defaultAction, defaultCondition }
