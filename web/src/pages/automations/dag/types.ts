import type { AutomationAction, AutomationCatalog, AutomationCondition } from '../../../api'

export type TriggerNodeData = {
  trigger: string
  readOnly: boolean
  triggers?: AutomationCatalog['triggers']
  onTriggerChange?: (trigger: string) => void
}

export type ConditionNodeData = {
  condition: AutomationCondition
  readOnly: boolean
  conditionFields?: AutomationCatalog['condition_fields']
  ops?: AutomationCatalog['ops']
  onChange?: (condition: AutomationCondition) => void
  onRemove?: () => void
  canRemove: boolean
}

export type ActionNodeData = {
  action: AutomationAction
  readOnly: boolean
  actions?: AutomationCatalog['actions']
  emailTemplates?: { key: string; name: string }[]
  onChange?: (action: AutomationAction) => void
  onRemove?: () => void
  canRemove: boolean
}

export type AutomationGraph = {
  trigger: string
  conditions: AutomationCondition[]
  actions: AutomationAction[]
}
