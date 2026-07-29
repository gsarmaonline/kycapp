import { TypographyField } from './TypographyField'
import { VariableDocsHint } from './VariableDocsHint'
import { newBodySection, type EmailBodySection, type RegionStyle } from '../email_fonts'

type Props = {
  sections: EmailBodySection[]
  onChange: (next: EmailBodySection[]) => void
  brandingHint?: boolean
}

export function EmailBodySectionsEditor({ sections, onChange, brandingHint }: Props) {
  function updateAt(i: number, patch: Partial<EmailBodySection>) {
    onChange(sections.map((s, idx) => (idx === i ? { ...s, ...patch } : s)))
  }

  function move(i: number, dir: -1 | 1) {
    const j = i + dir
    if (j < 0 || j >= sections.length) return
    const next = [...sections]
    ;[next[i], next[j]] = [next[j], next[i]]
    onChange(next)
  }

  return (
    <div className="email-body-sections">
      {brandingHint ? (
        <p className="field-hint">
          Each section inherits the body default style from Branding unless you override it below.
        </p>
      ) : null}
      {sections.map((sec, i) => (
        <fieldset key={sec.id} className="settings-block email-body-section">
          <legend>
            Body section {i + 1}
            <span className="email-section-actions">
              <button type="button" className="ghost" disabled={i === 0} onClick={() => move(i, -1)}>
                Up
              </button>
              <button
                type="button"
                className="ghost"
                disabled={i === sections.length - 1}
                onClick={() => move(i, 1)}
              >
                Down
              </button>
              <button
                type="button"
                className="ghost"
                disabled={sections.length <= 1}
                onClick={() => onChange(sections.filter((_, idx) => idx !== i))}
              >
                Remove
              </button>
            </span>
          </legend>
          <label>
            Content (HTML)
            <VariableDocsHint>
              Placeholders e.g. <code>{'{{app_user.display_name}}'}</code>.
            </VariableDocsHint>
            <textarea
              value={sec.content_html}
              onChange={(e) => updateAt(i, { content_html: e.target.value })}
              rows={5}
            />
          </label>
          <TypographyField
            label="Section style overrides"
            value={sec.style}
            partial
            onChange={(style) => updateAt(i, { style: style as Partial<RegionStyle> })}
          />
        </fieldset>
      ))}
      <button
        type="button"
        className="ghost"
        onClick={() => onChange([...sections, newBodySection('<p></p>')])}
      >
        Add body section
      </button>
    </div>
  )
}
