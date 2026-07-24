import { useCallback, useMemo } from 'react'
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { AutomationAction, AutomationCatalog, AutomationCondition } from '../../../api'
import { buildFlowElements, defaultAction, defaultCondition, normalizeGraph } from './build_graph'
import { automationNodeTypes } from './nodes'

type Props = {
  readOnly?: boolean
  catalog?: AutomationCatalog | null
  emailTemplates?: { key: string; name: string }[]
  trigger: string
  conditions: AutomationCondition[]
  actions: AutomationAction[]
  onTriggerChange?: (trigger: string) => void
  onConditionsChange?: (conditions: AutomationCondition[]) => void
  onActionsChange?: (actions: AutomationAction[]) => void
}

function preferredConditionField(catalog?: AutomationCatalog | null) {
  const attrs = catalog?.condition_fields?.find((f) => f.group === 'attributes')
  return attrs?.field ?? catalog?.condition_fields?.[0]?.field ?? 'status'
}

function preferredActionType(catalog?: AutomationCatalog | null) {
  return catalog?.actions?.[0]?.type ?? 'send_email'
}

function AutomationDagInner({
  readOnly = false,
  catalog,
  emailTemplates = [],
  trigger,
  conditions,
  actions,
  onTriggerChange,
  onConditionsChange,
  onActionsChange,
}: Props) {
  const graph = useMemo(
    () =>
      normalizeGraph(
        { trigger, conditions, actions },
        { ensureDefaults: !readOnly },
      ),
    [trigger, conditions, actions, readOnly],
  )

  const onConditionChange = useCallback(
    (index: number, condition: AutomationCondition) => {
      const next = [...graph.conditions]
      next[index] = condition
      onConditionsChange?.(next)
    },
    [graph.conditions, onConditionsChange],
  )

  const onConditionRemove = useCallback(
    (index: number) => {
      if (graph.conditions.length <= 1) return
      onConditionsChange?.(graph.conditions.filter((_, i) => i !== index))
    },
    [graph.conditions, onConditionsChange],
  )

  const onActionChange = useCallback(
    (index: number, action: AutomationAction) => {
      const next = [...graph.actions]
      next[index] = action
      onActionsChange?.(next)
    },
    [graph.actions, onActionsChange],
  )

  const onActionRemove = useCallback(
    (index: number) => {
      if (graph.actions.length <= 1) return
      onActionsChange?.(graph.actions.filter((_, i) => i !== index))
    },
    [graph.actions, onActionsChange],
  )

  const { nodes, edges } = useMemo(
    () =>
      buildFlowElements(graph, {
        readOnly,
        catalog,
        emailTemplates,
        onTriggerChange,
        onConditionChange,
        onConditionRemove,
        onActionChange,
        onActionRemove,
      }),
    [
      graph,
      readOnly,
      catalog,
      emailTemplates,
      onTriggerChange,
      onConditionChange,
      onConditionRemove,
      onActionChange,
      onActionRemove,
    ],
  )

  return (
    <div className={`automation-dag${readOnly ? ' is-readonly' : ''}`}>
      {!readOnly && (
        <div className="dag-toolbar">
          <button
            type="button"
            className="ghost"
            onClick={() =>
              onConditionsChange?.([
                ...graph.conditions,
                defaultCondition(preferredConditionField(catalog)),
              ])
            }
          >
            Add condition
          </button>
          <button
            type="button"
            className="ghost"
            onClick={() =>
              onActionsChange?.([
                ...graph.actions,
                defaultAction(preferredActionType(catalog), emailTemplates[0]?.key ?? ''),
              ])
            }
          >
            Add action
          </button>
          <span className="field-hint">
            All conditions must match (AND), then every action runs. Condition fields include all
            active user attributes.
          </span>
        </div>
      )}
      <div className="dag-canvas">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={automationNodeTypes}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={!readOnly}
          panOnScroll
          zoomOnScroll
          proOptions={{ hideAttribution: true }}
        >
          <Background gap={16} size={1} />
          <Controls showInteractive={false} />
          <MiniMap pannable zoomable />
        </ReactFlow>
      </div>
    </div>
  )
}

export function AutomationDag(props: Props) {
  return (
    <ReactFlowProvider>
      <AutomationDagInner {...props} />
    </ReactFlowProvider>
  )
}
