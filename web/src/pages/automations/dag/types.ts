import type { AutomationAction, AutomationCondition } from '../../../api'

export type TriggerNodeData = {
  trigger: string
  readOnly: boolean
  onTriggerChange?: (trigger: string) => void
}

export type ConditionNodeData = {
  condition: AutomationCondition
  readOnly: boolean
  onChange?: (condition: AutomationCondition) => void
  onRemove?: () => void
  canRemove: boolean
}

export type ActionNodeData = {
  action: AutomationAction
  readOnly: boolean
  onChange?: (action: AutomationAction) => void
  onRemove?: () => void
  canRemove: boolean
}

export type AutomationGraph = {
  trigger: string
  conditions: AutomationCondition[]
  actions: AutomationAction[]
}
