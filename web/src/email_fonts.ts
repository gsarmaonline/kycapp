/** Mirrors core/emailtemplates.FontStacks — email-safe stacks only. */

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

export function emailFontStack(key?: string): string {
  const found = EMAIL_FONTS.find((f) => f.key === (key || '').toLowerCase().trim())
  return found?.stack ?? EMAIL_FONTS[0].stack
}
