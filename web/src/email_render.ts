/** Matches core/emailtemplates.Render: {{var}} placeholders. */

const PLACEHOLDER_RE = /\{\{\s*([a-zA-Z0-9_]+)\s*\}\}/g

export function renderEmailTemplate(template: string, vars: Record<string, string>): string {
  if (!template) return template
  return template.replace(PLACEHOLDER_RE, (match, key: string) =>
    Object.prototype.hasOwnProperty.call(vars, key) ? vars[key] : match,
  )
}
