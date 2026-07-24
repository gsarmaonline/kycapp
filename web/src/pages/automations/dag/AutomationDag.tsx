import { useCallback, useMemo } from 'react'
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type {
  AutomationAction,
  AutomationCatalog,
  AutomationCondition,
  AutomationConditionMode,
} from '../../../api'
import { buildFlowElements, appendAction, defaultAction, defaultCondition, normalizeActionWorkflow, normalizeGraph, removeActionAt } from './build_graph'
import { automationNodeTypes } from './nodes'

type Props = {
  readOnly?: boolean
  catalog?: AutomationCatalog | null
  emailTemplates?: { key: string; name: string }[]
  trigger: string
  triggerParams?: Record<string, string>
  conditionMode?: AutomationConditionMode
  conditions: AutomationCondition[]
  actions: AutomationAction[]
  onTriggerChange?: (trigger: string) => void
  onTriggerParamsChange?: (params: Record<string, string>) => void
  onConditionModeChange?: (mode: AutomationConditionMode) => void
  onConditionsChange?: (conditions: AutomationCondition[]) => void
  onActionsChange?: (actions: AutomationAction[]) => void
}

function preferredConditionField(catalog?: AutomationCatalog | null) {
  const attrs = catalog?.condition_fields?.find((f) => f.group === 'attributes')
  return attrs?.field ?? catalog?.condition_fields?.[0]?.field ?? 'app_user.status'
}

function preferredActionType(catalog?: AutomationCatalog | null) {
  return catalog?.actions?.[0]?.type ?? 'send_email'
}

function AutomationDagInner({
  readOnly = false,
  catalog,
  emailTemplates = [],
  trigger,
  triggerParams = {},
  conditionMode = 'all',
  conditions,
  actions,
  onTriggerChange,
  onTriggerParamsChange,
  onConditionModeChange,
  onConditionsChange,
  onActionsChange,
}: Props) {
  const graph = useMemo(
    () =>
      normalizeGraph(
        { trigger, triggerParams, conditions, actions },
        { ensureDefaults: !readOnly },
      ),
    [trigger, triggerParams, conditions, actions, readOnly],
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
      onActionsChange?.(normalizeActionWorkflow(next))
    },
    [graph.actions, onActionsChange],
  )

  const onActionRemove = useCallback(
    (index: number) => {
      if (graph.actions.length <= 1) return
      onActionsChange?.(removeActionAt(graph.actions, index))
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
        onTriggerParamsChange,
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
      onTriggerParamsChange,
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
          <label className="dag-mode">
            Match
            <select
              value={conditionMode}
              onChange={(e) =>
                onConditionModeChange?.(e.target.value === 'any' ? 'any' : 'all')
              }
            >
              <option value="all">all conditions (AND)</option>
              <option value="any">any condition (OR)</option>
            </select>
          </label>
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
              onActionsChange?.(
                appendAction(
                  graph.actions,
                  defaultAction(preferredActionType(catalog), emailTemplates[0]?.key ?? ''),
                ),
              )
            }
          >
            Add action
          </button>
          <span className="field-hint">
            Actions run as a workflow: success follows the green path; set{' '}
            <em>On error</em> to branch instead of failing the run.
          </span>
        </div>
      )}
      {readOnly && (
        <p className="field-hint">
          Match {conditionMode === 'any' ? 'any' : 'all'} condition
          {graph.conditions.length === 1 ? '' : 's'} (
          {conditionMode === 'any' ? 'OR' : 'AND'}
          ). Green edges = on success; red = on error.
        </p>
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
