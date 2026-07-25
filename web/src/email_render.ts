/** Matches core/emailtemplates.Render / Wrap. */

import {
  emailFontStack,
  regionInlineCSS,
  resolveTypography,
  type EmailTypography,
} from './email_fonts'

const PLACEHOLDER_RE = /\{\{\s*([a-zA-Z0-9_]+)\s*\}\}/g

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

export function renderEmailTemplate(template: string, vars: Record<string, string>): string {
  if (!template) return template
  return template.replace(PLACEHOLDER_RE, (match, key: string) =>
    Object.prototype.hasOwnProperty.call(vars, key) ? vars[key] : match,
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
