/** Mirrors core/emailtemplates typography — email-safe stacks + region styles. */

export type EmailFontOption = {
  key: string
  label: string
  stack: string
}

export const EMAIL_FONTS: EmailFontOption[] = [
  { key: 'arial', label: 'Arial', stack: 'Arial, Helvetica, sans-serif' },
  { key: 'helvetica', label: 'Helvetica', stack: 'Helvetica, Arial, sans-serif' },
  { key: 'verdana', label: 'Verdana', stack: 'Verdana, Geneva, sans-serif' },
  { key: 'trebuchet', label: 'Trebuchet MS', stack: "'Trebuchet MS', Helvetica, sans-serif" },
  { key: 'georgia', label: 'Georgia', stack: "Georgia, 'Times New Roman', serif" },
  { key: 'times', label: 'Times New Roman', stack: "'Times New Roman', Times, serif" },
  { key: 'courier', label: 'Courier New', stack: "'Courier New', Courier, monospace" },
]

export const EMAIL_FONT_SIZES = [10, 11, 12, 13, 14, 15, 16, 18, 20, 22, 24, 28, 32] as const
export const EMAIL_FONT_WEIGHTS = [400, 500, 600, 700] as const
export const EMAIL_FONT_STYLES = ['normal', 'italic'] as const
export const EMAIL_TEXT_ALIGNS = ['left', 'center', 'right'] as const
export const EMAIL_PADDING_LEFT = [0, 4, 8, 12, 16, 20, 24, 32, 40, 48] as const

export type EmailFontStyle = (typeof EMAIL_FONT_STYLES)[number]
export type EmailTextAlign = (typeof EMAIL_TEXT_ALIGNS)[number]

export type RegionStyle = {
  font: string
  size: number
  weight: number
  style: EmailFontStyle
  text_color?: string
  background_color?: string
  text_align?: EmailTextAlign
  padding_left?: number
}

export type EmailTypography = {
  header: RegionStyle
  body: RegionStyle
  footer: RegionStyle
}

export type EmailBodySection = {
  id: string
  content_html: string
  style: Partial<RegionStyle>
}

export function emailFontStack(key?: string): string {
  const found = EMAIL_FONTS.find((f) => f.key === (key || '').toLowerCase().trim())
  return found?.stack ?? EMAIL_FONTS[0].stack
}

export function defaultTypography(fontKey = 'arial'): EmailTypography {
  const font = EMAIL_FONTS.some((f) => f.key === fontKey) ? fontKey : 'arial'
  return {
    header: { font, size: 20, weight: 700, style: 'normal', text_align: 'center' },
    body: { font, size: 16, weight: 400, style: 'normal', text_align: 'left' },
    footer: { font, size: 12, weight: 400, style: 'normal', text_align: 'center' },
  }
}

function mergeRegion(base: RegionStyle, partial?: Partial<RegionStyle> | null): RegionStyle {
  if (!partial) return base
  const style = partial.style === 'italic' || partial.style === 'normal' ? partial.style : base.style
  const align =
    partial.text_align === 'left' ||
    partial.text_align === 'center' ||
    partial.text_align === 'right'
      ? partial.text_align
      : base.text_align
  return {
    font: partial.font?.trim() || base.font,
    size: partial.size || base.size,
    weight: partial.weight || base.weight,
    style,
    text_color: partial.text_color?.trim() || base.text_color,
    background_color: partial.background_color?.trim() || base.background_color,
    text_align: align,
    padding_left: partial.padding_left || base.padding_left || 0,
  }
}

export function resolveTypography(
  raw?: Partial<EmailTypography> | null,
  legacyFont = 'arial',
): EmailTypography {
  const base = defaultTypography(legacyFont)
  if (!raw) return base
  return {
    header: mergeRegion(base.header, raw.header),
    body: mergeRegion(base.body, raw.body),
    footer: mergeRegion(base.footer, raw.footer),
  }
}

export function mergeSectionStyle(
  base: RegionStyle,
  partial?: Partial<RegionStyle> | null,
): RegionStyle {
  return mergeRegion(base, partial)
}

export function regionInlineCSS(r: RegionStyle): string {
  let css = `font-family:${emailFontStack(r.font)};font-size:${r.size}px;font-weight:${r.weight};font-style:${r.style};`
  if (r.text_color?.trim()) css += `color:${r.text_color.trim()};`
  if (r.background_color?.trim()) css += `background-color:${r.background_color.trim()};`
  css += `text-align:${r.text_align || 'left'};`
  if (r.padding_left && r.padding_left > 0) css += `padding-left:${r.padding_left}px;`
  return css
}

export function resolveFrom(
  tplFromName: string,
  tplFromAddress: string,
  orgFromName: string,
  orgFromAddress: string,
  envFrom = '',
): string {
  const name = [tplFromName, orgFromName].map((s) => s.trim()).find(Boolean) || ''
  const addr = [tplFromAddress, orgFromAddress].map((s) => s.trim()).find(Boolean) || ''
  if (addr) {
    if (!name) return addr
    if (addr.includes('<') && addr.includes('>')) return addr
    return `${name} <${addr}>`
  }
  return envFrom.trim()
}

export function newBodySection(contentHtml = ''): EmailBodySection {
  return {
    id: `sec_${Math.random().toString(36).slice(2, 10)}`,
    content_html: contentHtml,
    style: {},
  }
}

export function sectionsFromLegacyHtml(html: string): EmailBodySection[] {
  const trimmed = html.trim()
  if (!trimmed) return [newBodySection('')]
  return [{ id: 'sec_legacy', content_html: html, style: {} }]
}
