import type { AutomationAction, AutomationCatalog, AutomationCondition } from '../../../api'

export type TriggerNodeData = {
  trigger: string
  triggerParams?: Record<string, string>
  readOnly: boolean
  triggers?: AutomationCatalog['triggers']
  inboundWebhooks?: { id: string; name: string }[]
  plans?: { id: string; name: string; key?: string }[]
  roles?: { id: string; name: string; key?: string }[]
  onTriggerChange?: (trigger: string) => void
  onTriggerParamsChange?: (params: Record<string, string>) => void
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
  actionIndex?: number
  siblingActions?: AutomationAction[]
  readOnly: boolean
  actions?: AutomationCatalog['actions']
  emailTemplates?: { key: string; name: string }[]
  databases?: { id: string; name: string }[]
  webhooks?: { id: string; name: string }[]
  onChange?: (action: AutomationAction) => void
  onRemove?: () => void
  canRemove: boolean
}

export type AutomationGraph = {
  trigger: string
  triggerParams?: Record<string, string>
  conditions: AutomationCondition[]
  actions: AutomationAction[]
}
