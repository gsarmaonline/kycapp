/**
 * Shared variable / field-path library for the web app.
 * Mirrors core/automations Lookup, NormalizeFieldPath, RenderStringTemplate.
 * See docs/variables.md (in-app: Documentation → Variables).
 */

const PLACEHOLDER_RE = /\{\{\s*([^{}]+?)\s*\}\}/g

const APP_USER_CORE = new Set([
  'id',
  'email',
  'display_name',
  'status',
  'external_id',
  'attributes',
])

export type TemplateData = Record<string, unknown>

/** Canonical docs path inside an organisation workspace. */
export function variablesDocsPath(orgId: string): string {
  return `/orgs/${orgId}/docs/variables`
}

export function normalizeFieldPath(path: string): string {
  const p = path.trim()
  if (!p) return p
  if (p.startsWith('app_user.')) return p
  if (p === 'org_name') return 'organisation.name'
  if (p.startsWith('attributes.') && p.length > 'attributes.'.length) {
    return `app_user.${p.slice('attributes.'.length)}`
  }
  if (APP_USER_CORE.has(p)) return `app_user.${p}`
  return p
}

function lookupDirect(data: TemplateData, path: string): unknown {
  const parts = path.split('.')
  let cur: unknown = data
  for (const part of parts) {
    if (!cur || typeof cur !== 'object' || Array.isArray(cur)) return undefined
    cur = (cur as Record<string, unknown>)[part]
  }
  return cur
}

function lookupAppUserObject(obj: Record<string, unknown>, rest: string): unknown {
  if (rest === 'attributes') return obj.attributes
  if (APP_USER_CORE.has(rest)) return lookupDirect(obj, rest)
  if (rest in obj) return obj[rest]
  const attrs = obj.attributes
  if (attrs && typeof attrs === 'object' && !Array.isArray(attrs)) {
    return (attrs as Record<string, unknown>)[rest]
  }
  return undefined
}

/** Resolve a field path against event / email render data. */
export function lookupPath(data: TemplateData, path: string): unknown {
  const raw = path.trim()
  if (!raw) return undefined
  if (raw === 'payload') return data

  const direct = lookupDirect(data, raw)
  if (direct !== undefined) return direct

  const canon = normalizeFieldPath(raw)
  if (canon.startsWith('app_user.')) {
    const rest = canon.slice('app_user.'.length)
    const user = data.app_user
    if (user && typeof user === 'object' && !Array.isArray(user)) {
      const fromUser = lookupAppUserObject(user as Record<string, unknown>, rest)
      if (fromUser !== undefined) return fromUser
    }
    return lookupAppUserObject(data, rest)
  }
  if (canon !== raw) return lookupDirect(data, canon)
  return undefined
}

function stringifyTemplateValue(v: unknown): string {
  if (v == null) return ''
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  try {
    return JSON.stringify(v)
  } catch {
    return String(v)
  }
}

/** Replace {{path}} placeholders (emails, webhook string embedding preview). */
export function renderStringTemplate(template: string, data: TemplateData): string {
  if (!template) return template
  return template.replace(PLACEHOLDER_RE, (_match, path: string) =>
    stringifyTemplateValue(lookupPath(data, path)),
  )
}

/** Build email/webhook-style context (mirrors emailtemplates.RenderContext). */
export function buildRenderContext(input: {
  org_id: string
  org_name: string
  app_user?: Record<string, unknown> | null
  payload?: TemplateData | null
  trigger?: string
}): TemplateData {
  const data: TemplateData = { ...(input.payload ?? {}) }
  if (input.app_user) data.app_user = input.app_user
  data.organisation_id = input.org_id
  data.organisation = { id: input.org_id, name: input.org_name }
  if (input.trigger) data.trigger = input.trigger
  return data
}
