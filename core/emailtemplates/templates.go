package emailtemplates

import (
	"fmt"
	"regexp"
	"strings"
)

var keyRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Spec is a template definition used for seeding and create validation.
type Spec struct {
	Key         string
	Name        string
	Description string
	Subject     string
	BodyText    string
	BodyHTML    string
}

// Defaults returns the built-in system templates seeded per organisation.
func Defaults() []Spec {
	return []Spec{
		{
			Key:         "welcome",
			Name:        "Welcome",
			Description: "Sent when an app user joins",
			Subject:     "Welcome to {{org_name}}",
			BodyText:    "Hi {{display_name}},\n\nWelcome to {{org_name}}. We're glad you're here.\n",
			BodyHTML:    "<p>Hi {{display_name}},</p><p>Welcome to {{org_name}}. We're glad you're here.</p>",
		},
		{
			Key:         "payment_thank_you",
			Name:        "Payment thank you",
			Description: "Sent after a successful payment",
			Subject:     "Thank you for your payment",
			BodyText:    "Hi {{display_name}},\n\nThank you for your payment to {{org_name}}.\n",
			BodyHTML:    "<p>Hi {{display_name}},</p><p>Thank you for your payment to {{org_name}}.</p>",
		},
		{
			Key:         "profile_incomplete",
			Name:        "Profile incomplete",
			Description: "Reminder to finish profile details",
			Subject:     "Please complete your profile",
			BodyText:    "Hi {{display_name}},\n\nPlease complete your profile so we can finish setting things up.\n",
			BodyHTML:    "<p>Hi {{display_name}},</p><p>Please complete your profile so we can finish setting things up.</p>",
		},
	}
}

// ValidKey reports whether key is a stable slug for templates / workflows.
func ValidKey(key string) bool {
	return keyRE.MatchString(key)
}

// NormalizeKey trims and lowercases a template key.
func NormalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

// CreateFields is input for a custom (non-system) template.
type CreateFields struct {
	Key         string
	Name        string
	Description string
	Subject     string
	BodyText    string
	BodyHTML    string
}

// ValidateCreate normalizes and validates create fields.
// Returns a normalized CreateFields or an error message suitable for API validation.
func ValidateCreate(in CreateFields) (CreateFields, error) {
	out := CreateFields{
		Key:         NormalizeKey(in.Key),
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Subject:     strings.TrimSpace(in.Subject),
		BodyText:    in.BodyText,
		BodyHTML:    in.BodyHTML,
	}
	if !ValidKey(out.Key) {
		return CreateFields{}, fmt.Errorf("key must be lowercase snake_case (a-z, 0-9, _)")
	}
	if out.Name == "" {
		return CreateFields{}, fmt.Errorf("name is required")
	}
	if out.Subject == "" {
		return CreateFields{}, fmt.Errorf("subject is required")
	}
	if strings.TrimSpace(out.BodyText) == "" && strings.TrimSpace(out.BodyHTML) == "" {
		return CreateFields{}, fmt.Errorf("body_text or body_html is required")
	}
	return out, nil
}

// ValidateStatus returns an error message if status is not allowed.
func ValidateStatus(status string) error {
	switch strings.TrimSpace(status) {
	case "active", "archived":
		return nil
	default:
		return fmt.Errorf("status must be active or archived")
	}
}

var placeholderRE = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

// Render replaces {{var}} placeholders. Missing keys are left unchanged.
func Render(s string, vars map[string]string) string {
	if vars == nil || s == "" {
		return s
	}
	return placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := placeholderRE.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v, ok := vars[sub[1]]; ok {
			return v
		}
		return m
	})
}
