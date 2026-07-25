package emailtemplates

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RegionStyle is typography for one email chrome region.
type RegionStyle struct {
	Font   string `json:"font"`
	Size   int    `json:"size"`
	Weight int    `json:"weight"`
	Style  string `json:"style"` // normal | italic
}

// Typography holds header / body / footer styles for branded emails.
type Typography struct {
	Header RegionStyle `json:"header"`
	Body   RegionStyle `json:"body"`
	Footer RegionStyle `json:"footer"`
}

var allowedSizes = map[int]struct{}{
	10: {}, 11: {}, 12: {}, 13: {}, 14: {}, 15: {}, 16: {}, 18: {}, 20: {}, 22: {}, 24: {}, 28: {}, 32: {},
}

var allowedWeights = map[int]struct{}{
	400: {}, 500: {}, 600: {}, 700: {},
}

// DefaultTypography returns the shipped defaults, using fontKey for all regions.
func DefaultTypography(fontKey string) Typography {
	font := strings.ToLower(strings.TrimSpace(fontKey))
	if font == "" {
		font = "arial"
	}
	if _, err := NormalizeFont(font); err != nil {
		font = "arial"
	}
	return Typography{
		Header: RegionStyle{Font: font, Size: 20, Weight: 700, Style: "normal"},
		Body:   RegionStyle{Font: font, Size: 16, Weight: 400, Style: "normal"},
		Footer: RegionStyle{Font: font, Size: 12, Weight: 400, Style: "normal"},
	}
}

// ResolveTypography merges stored JSON with defaults (legacy email_font fills empty families).
func ResolveTypography(raw json.RawMessage, legacyFont string) Typography {
	base := DefaultTypography(legacyFont)
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return base
	}
	var partial Typography
	if err := json.Unmarshal(raw, &partial); err != nil {
		return base
	}
	return mergeTypography(base, partial)
}

func mergeTypography(base, partial Typography) Typography {
	out := base
	out.Header = mergeRegion(base.Header, partial.Header)
	out.Body = mergeRegion(base.Body, partial.Body)
	out.Footer = mergeRegion(base.Footer, partial.Footer)
	return out
}

func mergeRegion(base, partial RegionStyle) RegionStyle {
	out := base
	if strings.TrimSpace(partial.Font) != "" {
		out.Font = partial.Font
	}
	if partial.Size != 0 {
		out.Size = partial.Size
	}
	if partial.Weight != 0 {
		out.Weight = partial.Weight
	}
	if strings.TrimSpace(partial.Style) != "" {
		out.Style = partial.Style
	}
	return out
}

// NormalizeTypography validates and returns a complete typography object.
func NormalizeTypography(in Typography, legacyFont string) (Typography, error) {
	base := DefaultTypography(legacyFont)
	merged := mergeTypography(base, in)
	header, err := normalizeRegion(merged.Header, "header")
	if err != nil {
		return Typography{}, err
	}
	body, err := normalizeRegion(merged.Body, "body")
	if err != nil {
		return Typography{}, err
	}
	footer, err := normalizeRegion(merged.Footer, "footer")
	if err != nil {
		return Typography{}, err
	}
	return Typography{Header: header, Body: body, Footer: footer}, nil
}

func normalizeRegion(r RegionStyle, name string) (RegionStyle, error) {
	font, err := NormalizeFont(r.Font)
	if err != nil || font == "" {
		return RegionStyle{}, fmt.Errorf("%s font: unsupported email font", name)
	}
	if _, ok := allowedSizes[r.Size]; !ok {
		return RegionStyle{}, fmt.Errorf("%s size must be one of 10–16, 18, 20, 22, 24, 28, 32", name)
	}
	if _, ok := allowedWeights[r.Weight]; !ok {
		return RegionStyle{}, fmt.Errorf("%s weight must be 400, 500, 600, or 700", name)
	}
	style := strings.ToLower(strings.TrimSpace(r.Style))
	if style == "" {
		style = "normal"
	}
	switch style {
	case "normal", "italic":
	default:
		return RegionStyle{}, fmt.Errorf("%s style must be normal or italic", name)
	}
	return RegionStyle{Font: font, Size: r.Size, Weight: r.Weight, Style: style}, nil
}

// MarshalTypography encodes typography for storage.
func MarshalTypography(t Typography) (json.RawMessage, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// InlineCSS returns inline CSS for a region (font-family, size, weight, style).
func (r RegionStyle) InlineCSS() string {
	return fmt.Sprintf(
		"font-family:%s;font-size:%dpx;font-weight:%d;font-style:%s;",
		FontStack(r.Font),
		r.Size,
		r.Weight,
		r.Style,
	)
}
