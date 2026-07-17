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
<body style="margin:0;padding:0;background:#f4f4f5;font-family:Arial,Helvetica,sans-serif;color:#1c1917;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:24px 12px;">
  <tr>
    <td align="center">
      <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border-radius:8px;overflow:hidden;">
        <tr>
          <td style="background:%s;height:6px;font-size:0;line-height:0;">&nbsp;</td>
        </tr>
        <tr>
          <td style="padding:28px 28px 12px;text-align:center;">
            %s
            <div style="font-size:20px;font-weight:700;color:%s;letter-spacing:0.01em;">%s</div>
          </td>
        </tr>
        <tr>
          <td style="padding:8px 28px 28px;font-size:16px;line-height:1.55;color:#1c1917;text-align:left;">
            %s
          </td>
        </tr>
        <tr>
          <td style="padding:16px 28px;background:%s;color:#f8faf8;font-size:12px;line-height:1.4;text-align:center;">
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
		html.EscapeString(primary),
		logoBlock,
		html.EscapeString(accent),
		orgName,
		inner,
		html.EscapeString(primary),
		footer,
	)
}
