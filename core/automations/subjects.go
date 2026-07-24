package automations

import (
	"fmt"
	"strings"

	"github.com/gsarmaonline/kyc/core/resources"
)

// ValidateSubjectCompatibility ensures every action's required subjects can be
// supplied by the trigger (resource provides + declared relations).
func ValidateSubjectCompatibility(trigger string, actions []Action) error {
	for i, a := range actions {
		a = a.Normalize()
		h, ok := LookupActionHandler(a.Type)
		if !ok {
			return fmt.Errorf("actions[%d]: action type %q is not supported", i, a.Type)
		}
		required := h.Requires()
		if len(required) == 0 {
			continue
		}
		missing, err := resources.MissingSubjects(trigger, required)
		if err != nil {
			return fmt.Errorf("actions[%d]: %w", i, err)
		}
		if len(missing) > 0 {
			return fmt.Errorf(
				"actions[%d]: %s requires subject(s) [%s], but trigger %q cannot provide them",
				i, a.Type, strings.Join(missing, ", "), trigger,
			)
		}
	}
	return nil
}

// RequiredSubjects unions Requires() across actions.
func RequiredSubjects(actions []Action) []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range actions {
		h, ok := LookupActionHandler(a.Normalize().Type)
		if !ok {
			continue
		}
		for _, s := range h.Requires() {
			s = strings.TrimSpace(s)
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
