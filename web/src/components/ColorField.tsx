import { HexColorPicker } from 'react-colorful'

type Props = {
  label: string
  value: string
  onChange: (hex: string) => void
  placeholder?: string
}

/** Hex color field: saturation panel + editable hex input. */
export function ColorField({ label, value, onChange, placeholder = '#000000' }: Props) {
  const hex = normalizeHex(value)

  return (
    <div className="color-field">
      <span className="color-field-label">{label}</span>
      <div className="color-field-row">
        <HexColorPicker color={hex} onChange={onChange} />
        <div className="color-field-side">
          <span className="color-field-swatch" style={{ background: hex }} aria-hidden />
          <input
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onBlur={() => onChange(normalizeHex(value))}
            placeholder={placeholder}
            spellCheck={false}
            aria-label={`${label} hex`}
          />
        </div>
      </div>
    </div>
  )
}

export function normalizeHex(c: string): string {
  const s = c.trim()
  if (/^#[0-9a-fA-F]{6}$/.test(s)) return s.toLowerCase()
  if (/^#[0-9a-fA-F]{3}$/.test(s)) {
    const r = s[1]
    const g = s[2]
    const b = s[3]
    return `#${r}${r}${g}${g}${b}${b}`.toLowerCase()
  }
  if (/^[0-9a-fA-F]{6}$/.test(s)) return `#${s.toLowerCase()}`
  return '#1f4d3a'
}
