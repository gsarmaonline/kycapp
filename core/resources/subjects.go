package resources

import (
	"fmt"
	"strings"
)

// Subject kinds that actions can require and triggers can provide.
const (
	SubjectAppUser      = AppUser
	SubjectUser         = "user" // KYC operator (membership user)
	SubjectMembership   = Membership
	SubjectSubscription = Subscription
)

// Relation declares that a resource payload can resolve another subject.
type Relation struct {
	Subject string // target subject kind
	Via     string // payload field holding the foreign id (e.g. "user_id")
}

// Resource describes a domain object that can emit automation triggers.
type Resource struct {
	Key                string
	Label              string
	Lifecycles         []string // created / updated / deleted
	SupportsAttributes bool     // expand {resource}.attribute.{key}
	// Provides are subjects present on the event without further lookups.
	// The resource key itself is always implied; list extras here if needed.
	Provides []string
	// Relations are subjects resolvable from payload foreign keys.
	Relations []Relation
}

// Default returns the built-in resource catalog.
func Default() []Resource {
	return []Resource{
		{
			Key:                AppUser,
			Label:              "App user",
			Lifecycles:         []string{LifecycleCreated, LifecycleUpdated, LifecycleDeleted},
			SupportsAttributes: true,
			Provides:           []string{SubjectAppUser},
		},
		{
			Key:                Membership,
			Label:              "Membership",
			Lifecycles:         []string{LifecycleCreated, LifecycleUpdated, LifecycleDeleted},
			SupportsAttributes: false,
			Provides:           []string{SubjectMembership},
			Relations: []Relation{
				{Subject: SubjectUser, Via: "user_id"},
			},
		},
		{
			Key:                Subscription,
			Label:              "Subscription",
			Lifecycles:         []string{LifecycleCreated, LifecycleUpdated, LifecycleDeleted},
			SupportsAttributes: false,
			Provides:           []string{SubjectSubscription},
			// No app_user/user relation yet — subscription automations cannot
			// use send_email until we define how to resolve a recipient.
		},
	}
}

// AvailableSubjects returns subject kinds a trigger can supply (provides + relations).
func AvailableSubjects(triggerID string) ([]string, error) {
	tr, err := ParseTrigger(triggerID)
	if err != nil {
		return nil, err
	}
	res, ok := ByKey(tr.Resource)
	if !ok {
		return nil, fmt.Errorf("unknown resource %q", tr.Resource)
	}
	return res.availableSubjectKinds(), nil
}

func (r Resource) availableSubjectKinds() []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(r.Key)
	for _, s := range r.Provides {
		add(s)
	}
	for _, rel := range r.Relations {
		add(rel.Subject)
	}
	return out
}

// AvailableSubjectSet is a lookup form of AvailableSubjects.
func AvailableSubjectSet(triggerID string) (map[string]bool, error) {
	list, err := AvailableSubjects(triggerID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(list))
	for _, s := range list {
		out[s] = true
	}
	return out, nil
}

// MissingSubjects returns required subjects not available from the trigger.
func MissingSubjects(triggerID string, required []string) ([]string, error) {
	avail, err := AvailableSubjectSet(triggerID)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, r := range required {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if !avail[r] {
			missing = append(missing, r)
		}
	}
	return missing, nil
}
