package emailtemplates

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// Branding is org-level chrome applied around email body HTML at render time.
type Branding struct {
	OrgName      string
	LogoURL      string
	PrimaryColor string
	AccentColor  string
	Footer       string
	Font         string     // legacy single font key; used when Typography is empty
	Typography   Typography // per-region styles (preferred)
}

// FontOption is a selectable email-safe font stack.
type FontOption struct {
	Key   string
	Label string
	Stack string
}

// FontStacks lists fonts that work reliably across major email clients.
func FontStacks() []FontOption {
	return []FontOption{
		{Key: "arial", Label: "Arial", Stack: "Arial, Helvetica, sans-serif"},
		{Key: "helvetica", Label: "Helvetica", Stack: "Helvetica, Arial, sans-serif"},
		{Key: "verdana", Label: "Verdana", Stack: "Verdana, Geneva, sans-serif"},
		{Key: "trebuchet", Label: "Trebuchet MS", Stack: "'Trebuchet MS', Helvetica, sans-serif"},
		{Key: "georgia", Label: "Georgia", Stack: "Georgia, 'Times New Roman', serif"},
		{Key: "times", Label: "Times New Roman", Stack: "'Times New Roman', Times, serif"},
		{Key: "courier", Label: "Courier New", Stack: "'Courier New', Courier, monospace"},
	}
}

// NormalizeFont returns a known font key or an error.
func NormalizeFont(s string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if key == "" {
		return "", nil
	}
	for _, f := range FontStacks() {
		if f.Key == key {
			return key, nil
		}
	}
	return "", fmt.Errorf("unsupported email font")
}

// FontStack resolves a font key to a CSS font-family value.
func FontStack(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, f := range FontStacks() {
		if f.Key == key {
			return f.Stack
		}
	}
	return FontStacks()[0].Stack
}

var hexColorRE = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// NormalizeColor returns a trimmed hex color or an error.
func NormalizeColor(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if !hexColorRE.MatchString(s) {
		return "", fmt.Errorf("color must be #RGB or #RRGGBB")
	}
	return strings.ToLower(s), nil
}

// IsFullDocument reports whether content already includes a full HTML document.
func IsFullDocument(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	return strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html")
}

// Wrap surrounds inner HTML with a table-based branded header/footer.
// Full documents are returned unchanged. Empty content becomes a minimal empty body.
func Wrap(content string, b Branding) string {
	content = strings.TrimSpace(content)
	if IsFullDocument(content) {
		return content
	}
	primary := b.PrimaryColor
	if primary == "" {
		primary = "#1f4d3a"
	}
	accent := b.AccentColor
	if accent == "" {
		accent = primary
	}
	ty := b.Typography
	if ty.Header.Size == 0 && ty.Body.Size == 0 && ty.Footer.Size == 0 {
		ty = DefaultTypography(b.Font)
	} else {
		ty = mergeTypography(DefaultTypography(b.Font), ty)
	}
	bodyFont := FontStack(ty.Body.Font)

	headerStyle := ty.Header
	if headerStyle.TextColor == "" {
		headerStyle.TextColor = accent
	}
	footerStyle := ty.Footer
	if footerStyle.TextColor == "" {
		footerStyle.TextColor = "#f8faf8"
	}
	if footerStyle.BackgroundColor == "" {
		footerStyle.BackgroundColor = primary
	}
	bodyStyle := ty.Body
	if bodyStyle.TextColor == "" {
		bodyStyle.TextColor = "#1c1917"
	}

	headerCSS := headerStyle.InlineCSS()
	bodyCSS := bodyStyle.InlineCSS()
	footerCSS := footerStyle.InlineCSS()
	headerAlign := headerStyle.AlignAttr()
	bodyAlign := bodyStyle.AlignAttr()
	footerAlign := footerStyle.AlignAttr()

	orgName := html.EscapeString(strings.TrimSpace(b.OrgName))
	footer := html.EscapeString(strings.TrimSpace(b.Footer))
	if footer == "" && orgName != "" {
		footer = orgName
	}

	var logoBlock string
	if u := strings.TrimSpace(b.LogoURL); u != "" {
		logoBlock = fmt.Sprintf(
			`<img src="%s" alt="%s" width="140" style="display:block;max-width:140px;height:auto;border:0;margin:0 auto 12px;" />`,
			html.EscapeString(u),
			orgName,
		)
	}

	inner := content
	if inner == "" {
		inner = "&nbsp;"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>%s</title>
</head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:%s;color:#1c1917;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:24px 12px;font-family:%s;">
  <tr>
    <td align="center">
      <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border-radius:8px;overflow:hidden;font-family:%s;">
        <tr>
          <td style="background:%s;height:6px;font-size:0;line-height:0;">&nbsp;</td>
        </tr>
        <tr>
          <td align="%s" style="padding:28px 28px 12px;%s">
            %s
            <div style="letter-spacing:0.01em;">%s</div>
          </td>
        </tr>
        <tr>
          <td align="%s" style="padding:8px 28px 28px;%sline-height:1.55;">
            %s
          </td>
        </tr>
        <tr>
          <td align="%s" style="padding:16px 28px;%sline-height:1.4;">
            %s
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`,
		orgName,
		bodyFont, bodyFont, bodyFont,
		html.EscapeString(primary),
		headerAlign,
		headerCSS,
		logoBlock,
		orgName,
		bodyAlign,
		bodyCSS,
		inner,
		footerAlign,
		footerCSS,
		footer,
	)
}
