import {
  EMAIL_FONTS,
  EMAIL_FONT_SIZES,
  EMAIL_FONT_STYLES,
  EMAIL_FONT_WEIGHTS,
  emailFontStack,
  type RegionStyle,
} from '../email_fonts'

type Props = {
  label: string
  value: RegionStyle
  onChange: (next: RegionStyle) => void
}

export function TypographyField({ label, value, onChange }: Props) {
  return (
    <fieldset className="typography-field">
      <legend>{label}</legend>
      <div className="typography-field-row">
        <label>
          Font
          <select
            value={value.font}
            onChange={(e) => onChange({ ...value, font: e.target.value })}
            aria-label={`${label} font`}
          >
            {EMAIL_FONTS.map((f) => (
              <option key={f.key} value={f.key} style={{ fontFamily: f.stack }}>
                {f.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Size
          <select
            value={value.size}
            onChange={(e) => onChange({ ...value, size: Number(e.target.value) })}
            aria-label={`${label} size`}
          >
            {EMAIL_FONT_SIZES.map((n) => (
              <option key={n} value={n}>
                {n}px
              </option>
            ))}
          </select>
        </label>
        <label>
          Weight
          <select
            value={value.weight}
            onChange={(e) => onChange({ ...value, weight: Number(e.target.value) })}
            aria-label={`${label} weight`}
          >
            {EMAIL_FONT_WEIGHTS.map((n) => (
              <option key={n} value={n}>
                {n === 400 ? 'Regular' : n === 500 ? 'Medium' : n === 600 ? 'Semibold' : 'Bold'}
              </option>
            ))}
          </select>
        </label>
        <label>
          Style
          <select
            value={value.style}
            onChange={(e) =>
              onChange({ ...value, style: e.target.value === 'italic' ? 'italic' : 'normal' })
            }
            aria-label={`${label} style`}
          >
            {EMAIL_FONT_STYLES.map((s) => (
              <option key={s} value={s}>
                {s === 'normal' ? 'Normal' : 'Italic'}
              </option>
            ))}
          </select>
        </label>
      </div>
      <p
        className="typography-preview"
        style={{
          fontFamily: emailFontStack(value.font),
          fontSize: `${value.size}px`,
          fontWeight: value.weight,
          fontStyle: value.style,
        }}
      >
        The quick brown fox
      </p>
    </fieldset>
  )
}
