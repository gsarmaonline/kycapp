package emailtemplates

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RegionStyle is typography and layout for one email chrome region or body section.
type RegionStyle struct {
	Font            string `json:"font"`
	Size            int    `json:"size"`
	Weight          int    `json:"weight"`
	Style           string `json:"style"` // normal | italic
	TextColor       string `json:"text_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	TextAlign       string `json:"text_align,omitempty"` // left | center | right
	PaddingLeft     int    `json:"padding_left,omitempty"`
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

var allowedAligns = map[string]struct{}{
	"left": {}, "center": {}, "right": {},
}

var allowedPaddingLeft = map[int]struct{}{
	0: {}, 4: {}, 8: {}, 12: {}, 16: {}, 20: {}, 24: {}, 32: {}, 40: {}, 48: {},
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
		Header: RegionStyle{Font: font, Size: 20, Weight: 700, Style: "normal", TextAlign: "center"},
		Body:   RegionStyle{Font: font, Size: 16, Weight: 400, Style: "normal", TextAlign: "left"},
		Footer: RegionStyle{Font: font, Size: 12, Weight: 400, Style: "normal", TextAlign: "center"},
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
	if strings.TrimSpace(partial.TextColor) != "" {
		out.TextColor = partial.TextColor
	}
	if strings.TrimSpace(partial.BackgroundColor) != "" {
		out.BackgroundColor = partial.BackgroundColor
	}
	if strings.TrimSpace(partial.TextAlign) != "" {
		out.TextAlign = partial.TextAlign
	}
	// Non-zero padding overrides; zero means "inherit" for partial section styles.
	if partial.PaddingLeft != 0 {
		out.PaddingLeft = partial.PaddingLeft
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
	textColor, err := NormalizeColor(r.TextColor)
	if err != nil {
		return RegionStyle{}, fmt.Errorf("%s text_color: %w", name, err)
	}
	bgColor, err := NormalizeColor(r.BackgroundColor)
	if err != nil {
		return RegionStyle{}, fmt.Errorf("%s background_color: %w", name, err)
	}
	align := strings.ToLower(strings.TrimSpace(r.TextAlign))
	if align == "" {
		align = "left"
	}
	if _, ok := allowedAligns[align]; !ok {
		return RegionStyle{}, fmt.Errorf("%s text_align must be left, center, or right", name)
	}
	pad := r.PaddingLeft
	if _, ok := allowedPaddingLeft[pad]; !ok {
		return RegionStyle{}, fmt.Errorf("%s padding_left must be one of 0, 4, 8, 12, 16, 20, 24, 32, 40, 48", name)
	}
	return RegionStyle{
		Font:            font,
		Size:            r.Size,
		Weight:          r.Weight,
		Style:           style,
		TextColor:       textColor,
		BackgroundColor: bgColor,
		TextAlign:       align,
		PaddingLeft:     pad,
	}, nil
}

// MarshalTypography encodes typography for storage.
func MarshalTypography(t Typography) (json.RawMessage, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// InlineCSS returns inline CSS for a region (font, size, weight, style, colors, align, indent).
func (r RegionStyle) InlineCSS() string {
	var b strings.Builder
	fmt.Fprintf(&b, "font-family:%s;font-size:%dpx;font-weight:%d;font-style:%s;",
		FontStack(r.Font), r.Size, r.Weight, r.Style)
	if c := strings.TrimSpace(r.TextColor); c != "" {
		fmt.Fprintf(&b, "color:%s;", c)
	}
	if c := strings.TrimSpace(r.BackgroundColor); c != "" {
		fmt.Fprintf(&b, "background-color:%s;", c)
	}
	align := strings.TrimSpace(r.TextAlign)
	if align == "" {
		align = "left"
	}
	fmt.Fprintf(&b, "text-align:%s;", align)
	if r.PaddingLeft > 0 {
		fmt.Fprintf(&b, "padding-left:%dpx;", r.PaddingLeft)
	}
	return b.String()
}

// AlignAttr returns HTML align= value for table cells.
func (r RegionStyle) AlignAttr() string {
	align := strings.ToLower(strings.TrimSpace(r.TextAlign))
	switch align {
	case "center", "right":
		return align
	default:
		return "left"
	}
}
