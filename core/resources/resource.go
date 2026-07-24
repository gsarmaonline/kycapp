package resources

import "strings"

// Lifecycle events shared across resources.
const (
	LifecycleCreated = "created"
	LifecycleUpdated = "updated"
	LifecycleDeleted = "deleted"
)

// Well-known resource keys.
const (
	AppUser      = "app_user"
	Membership   = "membership"
	Subscription = "subscription"
)

// AttributeSegment is the fixed path segment between resource and attribute key.
const AttributeSegment = "attribute"

// AttributeKey is a dynamic field that can become an attribute trigger.
type AttributeKey struct {
	Key   string
	Label string
}

// Resource describes a domain object that can emit automation triggers.
type Resource struct {
	Key                string
	Label              string
	Lifecycles         []string // created / updated / deleted
	SupportsAttributes bool     // expand {resource}.attribute.{key}
}

// Default returns the built-in resource catalog.
func Default() []Resource {
	return []Resource{
		{
			Key:                AppUser,
			Label:              "App user",
			Lifecycles:         []string{LifecycleCreated, LifecycleUpdated, LifecycleDeleted},
			SupportsAttributes: true,
		},
		{
			Key:                Membership,
			Label:              "Membership",
			Lifecycles:         []string{LifecycleCreated, LifecycleUpdated, LifecycleDeleted},
			SupportsAttributes: false,
		},
		{
			Key:                Subscription,
			Label:              "Subscription",
			Lifecycles:         []string{LifecycleCreated, LifecycleUpdated, LifecycleDeleted},
			SupportsAttributes: false,
		},
	}
}

// ByKey looks up a registered resource.
func ByKey(key string) (Resource, bool) {
	key = strings.TrimSpace(key)
	for _, r := range Default() {
		if r.Key == key {
			return r, true
		}
	}
	return Resource{}, false
}

// HasLifecycle reports whether the resource declares the lifecycle event.
func (r Resource) HasLifecycle(event string) bool {
	for _, e := range r.Lifecycles {
		if e == event {
			return true
		}
	}
	return false
}
