/** Matches core/emailtemplates.Wrap; string render uses shared templates/paths. */

import {
  emailFontStack,
  mergeSectionStyle,
  regionInlineCSS,
  resolveTypography,
  type EmailBodySection,
  type EmailTypography,
  type RegionStyle,
} from './email_fonts'
import {
  buildRenderContext,
  renderStringTemplate,
  type TemplateData,
} from './templates/paths'

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

export type EmailRenderData = TemplateData

export { buildRenderContext as emailRenderContext, renderStringTemplate as renderEmailTemplate }
export { resolveFrom } from './email_fonts'

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

function alignAttr(r: RegionStyle): string {
  const a = (r.text_align || 'left').toLowerCase()
  return a === 'center' || a === 'right' ? a : 'left'
}

/** Compose body sections into inner HTML (mirrors ComposeBodyHTML). */
export function composeBodySectionsHtml(
  sections: EmailBodySection[],
  brandingBodyDefault: RegionStyle,
  data: Record<string, unknown>,
): string {
  if (!sections.length) return ''
  return sections
    .map((sec, i) => {
      const content = renderStringTemplate(sec.content_html || '', data)
      const style = mergeSectionStyle(brandingBodyDefault, sec.style)
      const gap =
        i > 0 ? `<div style="height:12px;line-height:12px;font-size:0;">&nbsp;</div>` : ''
      return `${gap}<div style="${regionInlineCSS(style)}">${content}</div>`
    })
    .join('')
}

/** Mirrors core/emailtemplates.Wrap — branded chrome around inner HTML. */
export function wrapEmailHtml(content: string, branding: EmailBranding): string {
  const trimmed = (content || '').trim()
  if (isFullEmailDocument(trimmed)) return trimmed

  const primary = branding.primary_color?.trim() || '#1f4d3a'
  const accent = branding.accent_color?.trim() || primary
  const ty = resolveTypography(branding.typography, branding.font || 'arial')
  const bodyFont = emailFontStack(ty.body.font)

  const headerStyle: RegionStyle = {
    ...ty.header,
    text_color: ty.header.text_color || accent,
  }
  const footerStyle: RegionStyle = {
    ...ty.footer,
    text_color: ty.footer.text_color || '#f8faf8',
    background_color: ty.footer.background_color || primary,
  }
  const bodyStyle: RegionStyle = {
    ...ty.body,
    text_color: ty.body.text_color || '#1c1917',
  }

  const headerCSS = regionInlineCSS(headerStyle)
  const bodyCSS = regionInlineCSS(bodyStyle)
  const footerCSS = regionInlineCSS(footerStyle)
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
          <td align="${alignAttr(headerStyle)}" style="padding:28px 28px 12px;${headerCSS}">
            ${logoBlock}
            <div style="letter-spacing:0.01em;">${orgName}</div>
          </td>
        </tr>
        <tr>
          <td align="${alignAttr(bodyStyle)}" style="padding:8px 28px 28px;${bodyCSS}line-height:1.55;">
            ${inner}
          </td>
        </tr>
        <tr>
          <td align="${alignAttr(footerStyle)}" style="padding:16px 28px;${footerCSS}line-height:1.4;">
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
