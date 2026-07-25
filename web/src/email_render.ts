/** Matches core/emailtemplates.Render / Wrap and automations.RenderStringTemplate. */

import {
  emailFontStack,
  regionInlineCSS,
  resolveTypography,
  type EmailTypography,
} from './email_fonts'

const PLACEHOLDER_RE = /\{\{\s*([^{}]+?)\s*\}\}/g

export type EmailBranding = {
  org_name: string
  logo_url?: string
  primary_color?: string
  accent_color?: string
  footer?: string
  /** @deprecated Prefer typography; still used as fallback font for all regions. */
  font?: string
  typography?: EmailTypography | null
}

export type EmailRenderData = Record<string, unknown>

const APP_USER_CORE = new Set([
  'id',
  'email',
  'display_name',
  'status',
  'external_id',
  'attributes',
])

function normalizeFieldPath(path: string): string {
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

function lookupDirect(data: EmailRenderData, path: string): unknown {
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

/** Mirrors automations.Lookup for preview rendering. */
export function lookupPath(data: EmailRenderData, path: string): unknown {
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

/** Shared render context for email previews (mirrors emailtemplates.RenderContext). */
export function emailRenderContext(input: {
  org_id: string
  org_name: string
  app_user?: Record<string, unknown> | null
  payload?: EmailRenderData | null
}): EmailRenderData {
  const data: EmailRenderData = { ...(input.payload ?? {}) }
  if (input.app_user) data.app_user = input.app_user
  data.organisation_id = input.org_id
  data.organisation = { id: input.org_id, name: input.org_name }
  return data
}

/** Mirrors automations.RenderStringTemplate / emailtemplates.Render. */
export function renderEmailTemplate(template: string, data: EmailRenderData): string {
  if (!template) return template
  return template.replace(PLACEHOLDER_RE, (_match, path: string) =>
    stringifyTemplateValue(lookupPath(data, path)),
  )
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

export function isFullEmailDocument(content: string): boolean {
  const lower = content.trim().toLowerCase()
  return lower.startsWith('<!doctype') || lower.startsWith('<html')
}

/** Mirrors core/emailtemplates.Wrap — branded chrome around inner HTML. */
export function wrapEmailHtml(content: string, branding: EmailBranding): string {
  const trimmed = (content || '').trim()
  if (isFullEmailDocument(trimmed)) return trimmed

  const primary = branding.primary_color?.trim() || '#1f4d3a'
  const accent = branding.accent_color?.trim() || primary
  const ty = resolveTypography(branding.typography, branding.font || 'arial')
  const bodyFont = emailFontStack(ty.body.font)
  const headerCSS = regionInlineCSS(ty.header)
  const bodyCSS = regionInlineCSS(ty.body)
  const footerCSS = regionInlineCSS(ty.footer)
  const orgName = escapeHtml((branding.org_name || '').trim())
  let footer = escapeHtml((branding.footer || '').trim())
  if (!footer && orgName) footer = orgName

  const logoURL = (branding.logo_url || '').trim()
  const logoBlock = logoURL
    ? `<img src="${escapeHtml(logoURL)}" alt="${orgName}" width="140" style="display:block;max-width:140px;height:auto;border:0;margin:0 auto 12px;" />`
    : ''

  const inner = trimmed || '&nbsp;'

  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>${orgName}</title>
</head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:${bodyFont};color:#1c1917;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:24px 12px;font-family:${bodyFont};">
  <tr>
    <td align="center">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border-radius:8px;overflow:hidden;font-family:${bodyFont};">
        <tr>
          <td style="background:${escapeHtml(primary)};height:6px;font-size:0;line-height:0;">&nbsp;</td>
        </tr>
        <tr>
          <td style="padding:28px 28px 12px;text-align:center;font-family:${bodyFont};">
            ${logoBlock}
            <div style="${headerCSS}color:${escapeHtml(accent)};letter-spacing:0.01em;">${orgName}</div>
          </td>
        </tr>
        <tr>
          <td style="padding:8px 28px 28px;${bodyCSS}line-height:1.55;color:#1c1917;text-align:left;">
            ${inner}
          </td>
        </tr>
        <tr>
          <td style="padding:16px 28px;background:${escapeHtml(primary)};color:#f8faf8;${footerCSS}line-height:1.4;text-align:center;">
            ${footer}
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`
}
