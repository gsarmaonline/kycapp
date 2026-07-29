package emailtemplates

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BodySection is one styled block inside an email template body.
type BodySection struct {
	ID          string      `json:"id"`
	ContentHTML string      `json:"content_html"`
	Style       RegionStyle `json:"style"`
}

const maxBodySections = 20

// NormalizeBodySections validates and normalizes a section list.
// Empty style fields are left empty so they inherit branding body defaults at render time.
func NormalizeBodySections(in []BodySection) ([]BodySection, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("at least one body section is required")
	}
	if len(in) > maxBodySections {
		return nil, fmt.Errorf("at most %d body sections allowed", maxBodySections)
	}
	out := make([]BodySection, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i, s := range in {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			return nil, fmt.Errorf("body section %d: id is required", i+1)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("body section id %q is duplicated", id)
		}
		seen[id] = struct{}{}
		style, err := normalizePartialRegion(s.Style, fmt.Sprintf("body section %d", i+1))
		if err != nil {
			return nil, err
		}
		out = append(out, BodySection{
			ID:          id,
			ContentHTML: s.ContentHTML,
			Style:       style,
		})
	}
	return out, nil
}

// normalizePartialRegion validates only fields that are set (for section overrides).
func normalizePartialRegion(r RegionStyle, name string) (RegionStyle, error) {
	out := RegionStyle{}
	if font := strings.TrimSpace(r.Font); font != "" {
		f, err := NormalizeFont(font)
		if err != nil || f == "" {
			return RegionStyle{}, fmt.Errorf("%s font: unsupported email font", name)
		}
		out.Font = f
	}
	if r.Size != 0 {
		if _, ok := allowedSizes[r.Size]; !ok {
			return RegionStyle{}, fmt.Errorf("%s size must be one of 10–16, 18, 20, 22, 24, 28, 32", name)
		}
		out.Size = r.Size
	}
	if r.Weight != 0 {
		if _, ok := allowedWeights[r.Weight]; !ok {
			return RegionStyle{}, fmt.Errorf("%s weight must be 400, 500, 600, or 700", name)
		}
		out.Weight = r.Weight
	}
	if style := strings.ToLower(strings.TrimSpace(r.Style)); style != "" {
		switch style {
		case "normal", "italic":
			out.Style = style
		default:
			return RegionStyle{}, fmt.Errorf("%s style must be normal or italic", name)
		}
	}
	if c := strings.TrimSpace(r.TextColor); c != "" {
		nc, err := NormalizeColor(c)
		if err != nil {
			return RegionStyle{}, fmt.Errorf("%s text_color: %w", name, err)
		}
		out.TextColor = nc
	}
	if c := strings.TrimSpace(r.BackgroundColor); c != "" {
		nc, err := NormalizeColor(c)
		if err != nil {
			return RegionStyle{}, fmt.Errorf("%s background_color: %w", name, err)
		}
		out.BackgroundColor = nc
	}
	if align := strings.ToLower(strings.TrimSpace(r.TextAlign)); align != "" {
		if _, ok := allowedAligns[align]; !ok {
			return RegionStyle{}, fmt.Errorf("%s text_align must be left, center, or right", name)
		}
		out.TextAlign = align
	}
	if r.PaddingLeft != 0 {
		if _, ok := allowedPaddingLeft[r.PaddingLeft]; !ok {
			return RegionStyle{}, fmt.Errorf("%s padding_left must be one of 0, 4, 8, 12, 16, 20, 24, 32, 40, 48", name)
		}
		out.PaddingLeft = r.PaddingLeft
	}
	return out, nil
}

// ParseBodySections unmarshals stored JSON into sections.
func ParseBodySections(raw json.RawMessage) ([]BodySection, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return nil, nil
	}
	var sections []BodySection
	if err := json.Unmarshal(raw, &sections); err != nil {
		return nil, fmt.Errorf("invalid body_sections: %w", err)
	}
	return sections, nil
}

// MarshalBodySections encodes sections for storage.
func MarshalBodySections(sections []BodySection) (json.RawMessage, error) {
	b, err := json.Marshal(sections)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// ComposeBodyHTML joins rendered sections into inner HTML for Wrap.
// Each section's style is merged onto brandingBodyDefault before applying a wrapper div.
func ComposeBodyHTML(sections []BodySection, brandingBodyDefault RegionStyle, data map[string]any) string {
	if len(sections) == 0 {
		return ""
	}
	var b strings.Builder
	for i, sec := range sections {
		content := Render(sec.ContentHTML, data)
		style := mergeRegion(brandingBodyDefault, sec.Style)
		if i > 0 {
			b.WriteString(`<div style="height:12px;line-height:12px;font-size:0;">&nbsp;</div>`)
		}
		fmt.Fprintf(&b, `<div style="%s">%s</div>`, style.InlineCSS(), content)
	}
	return b.String()
}

// BodySectionsOrLegacy returns body_sections when present, otherwise a single section from body_html.
func BodySectionsOrLegacy(raw json.RawMessage, bodyHTML string) ([]BodySection, error) {
	sections, err := ParseBodySections(raw)
	if err != nil {
		return nil, err
	}
	if len(sections) > 0 {
		return sections, nil
	}
	html := strings.TrimSpace(bodyHTML)
	if html == "" {
		return nil, nil
	}
	return []BodySection{{ID: "legacy", ContentHTML: bodyHTML}}, nil
}

// SyncBodyHTMLFromSections concatenates section HTML for the legacy body_html column.
func SyncBodyHTMLFromSections(sections []BodySection) string {
	if len(sections) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sections))
	for _, s := range sections {
		if strings.TrimSpace(s.ContentHTML) != "" {
			parts = append(parts, s.ContentHTML)
		}
	}
	return strings.Join(parts, "\n")
}

// ResolveFrom builds a From header with fallback: template → org → env.
// Name and address are treated leniently; empty values fall through the chain.
func ResolveFrom(tplFromName, tplFromAddress, orgFromName, orgFromAddress, envFrom string) string {
	name := firstNonEmpty(tplFromName, orgFromName)
	addr := firstNonEmpty(tplFromAddress, orgFromAddress)
	if addr != "" {
		name = strings.TrimSpace(name)
		addr = strings.TrimSpace(addr)
		if name == "" {
			return addr
		}
		// If address already looks like "Name <email>", prefer it as-is when name empty above;
		// when name is set and addr has no angle brackets, format as Name <addr>.
		if strings.Contains(addr, "<") && strings.Contains(addr, ">") {
			return addr
		}
		return fmt.Sprintf("%s <%s>", name, addr)
	}
	return strings.TrimSpace(envFrom)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// NormalizeFromFields trims from name/address. Lenient: any non-empty string is accepted.
func NormalizeFromFields(name, address string) (string, string) {
	return strings.TrimSpace(name), strings.TrimSpace(address)
}

// SectionFromBodyHTML builds a default single section from legacy HTML.
func SectionFromBodyHTML(id, html string) BodySection {
	return BodySection{ID: id, ContentHTML: html, Style: RegionStyle{}}
}
