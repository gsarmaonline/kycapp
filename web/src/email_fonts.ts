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

export type EmailFontStyle = (typeof EMAIL_FONT_STYLES)[number]

export type RegionStyle = {
  font: string
  size: number
  weight: number
  style: EmailFontStyle
}

export type EmailTypography = {
  header: RegionStyle
  body: RegionStyle
  footer: RegionStyle
}

export function emailFontStack(key?: string): string {
  const found = EMAIL_FONTS.find((f) => f.key === (key || '').toLowerCase().trim())
  return found?.stack ?? EMAIL_FONTS[0].stack
}

export function defaultTypography(fontKey = 'arial'): EmailTypography {
  const font = EMAIL_FONTS.some((f) => f.key === fontKey) ? fontKey : 'arial'
  return {
    header: { font, size: 20, weight: 700, style: 'normal' },
    body: { font, size: 16, weight: 400, style: 'normal' },
    footer: { font, size: 12, weight: 400, style: 'normal' },
  }
}

function mergeRegion(base: RegionStyle, partial?: Partial<RegionStyle> | null): RegionStyle {
  if (!partial) return base
  const style = partial.style === 'italic' || partial.style === 'normal' ? partial.style : base.style
  return {
    font: partial.font?.trim() || base.font,
    size: partial.size || base.size,
    weight: partial.weight || base.weight,
    style,
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

export function regionInlineCSS(r: RegionStyle): string {
  return `font-family:${emailFontStack(r.font)};font-size:${r.size}px;font-weight:${r.weight};font-style:${r.style};`
}
