import { ColorField, normalizeHex } from './ColorField'
import {
  EMAIL_FONTS,
  EMAIL_FONT_SIZES,
  EMAIL_FONT_STYLES,
  EMAIL_FONT_WEIGHTS,
  EMAIL_PADDING_LEFT,
  EMAIL_TEXT_ALIGNS,
  emailFontStack,
  type RegionStyle,
} from '../email_fonts'

type Props = {
  label: string
  value: Partial<RegionStyle>
  onChange: (next: Partial<RegionStyle>) => void
  /** When true, empty color/align fields are allowed (section overrides). */
  partial?: boolean
}

/** Typography + color/align/indent controls for an email region or body section. */
export function TypographyField({ label, value, onChange, partial = false }: Props) {
  return (
    <fieldset className="typography-field">
      <legend>{label}</legend>
      <div className="typography-field-row">
        <label>
          Font
          <select
            value={value.font || ''}
            onChange={(e) => onChange({ ...value, font: e.target.value || undefined })}
            aria-label={`${label} font`}
          >
            {partial ? <option value="">Inherit</option> : null}
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
            value={value.size || 0}
            onChange={(e) => {
              const n = Number(e.target.value)
              onChange({ ...value, size: n || undefined })
            }}
            aria-label={`${label} size`}
          >
            {partial ? <option value={0}>Inherit</option> : null}
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
            value={value.weight || 0}
            onChange={(e) => {
              const n = Number(e.target.value)
              onChange({ ...value, weight: n || undefined })
            }}
            aria-label={`${label} weight`}
          >
            {partial ? <option value={0}>Inherit</option> : null}
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
            value={value.style || ''}
            onChange={(e) => {
              const v = e.target.value
              onChange({
                ...value,
                style: v === 'italic' || v === 'normal' ? v : undefined,
              })
            }}
            aria-label={`${label} style`}
          >
            {partial ? <option value="">Inherit</option> : null}
            {EMAIL_FONT_STYLES.map((s) => (
              <option key={s} value={s}>
                {s === 'normal' ? 'Normal' : 'Italic'}
              </option>
            ))}
          </select>
        </label>
        <label>
          Align
          <select
            value={value.text_align || ''}
            onChange={(e) => {
              const v = e.target.value
              onChange({
                ...value,
                text_align: v === 'left' || v === 'center' || v === 'right' ? v : undefined,
              })
            }}
            aria-label={`${label} align`}
          >
            {partial ? <option value="">Inherit</option> : null}
            {EMAIL_TEXT_ALIGNS.map((a) => (
              <option key={a} value={a}>
                {a[0].toUpperCase() + a.slice(1)}
              </option>
            ))}
          </select>
        </label>
        <label>
          Indent
          <select
            value={value.padding_left ?? 0}
            onChange={(e) => {
              const n = Number(e.target.value)
              onChange({ ...value, padding_left: n || undefined })
            }}
            aria-label={`${label} indent`}
          >
            {EMAIL_PADDING_LEFT.map((n) => (
              <option key={n} value={n}>
                {n === 0 ? (partial ? 'Inherit / none' : 'None') : `${n}px`}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="typography-field-colors">
        <ColorField
          label="Font color"
          value={value.text_color || '#1c1917'}
          onChange={(hex) => onChange({ ...value, text_color: normalizeHex(hex) })}
        />
        {partial ? (
          <button
            type="button"
            className="ghost"
            onClick={() => onChange({ ...value, text_color: undefined })}
          >
            Clear font color
          </button>
        ) : null}
        <ColorField
          label="Background"
          value={value.background_color || '#ffffff'}
          onChange={(hex) => onChange({ ...value, background_color: normalizeHex(hex) })}
        />
        {partial ? (
          <button
            type="button"
            className="ghost"
            onClick={() => onChange({ ...value, background_color: undefined })}
          >
            Clear background
          </button>
        ) : null}
      </div>
      <p
        className="typography-preview"
        style={{
          fontFamily: emailFontStack(value.font || 'arial'),
          fontSize: `${value.size || 16}px`,
          fontWeight: value.weight || 400,
          fontStyle: value.style || 'normal',
          color: value.text_color || undefined,
          backgroundColor: value.background_color || undefined,
          textAlign: value.text_align || 'left',
          paddingLeft: value.padding_left ? `${value.padding_left}px` : undefined,
        }}
      >
        The quick brown fox
      </p>
    </fieldset>
  )
}
