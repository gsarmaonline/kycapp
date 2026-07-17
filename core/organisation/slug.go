package organisation

import (
	"regexp"
	"strings"
	"unicode"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9-]+`)

// Slugify turns a display name into a URL-safe slug.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '_' || r == '-':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	out = nonSlug.ReplaceAllString(out, "")
	if out == "" {
		return "org"
	}
	return out
}
