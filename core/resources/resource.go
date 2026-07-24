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
	Schedule     = "schedule" // org-scoped time triggers (not per app_user)
)

// Schedule presets (used as lifecycle events on the schedule resource).
const (
	ScheduleHourly = "hourly"
	ScheduleDaily  = "daily"
	ScheduleWeekly = "weekly"
)

// AttributeSegment is the fixed path segment between resource and attribute key.
const AttributeSegment = "attribute"

// AttributeKey is a dynamic field that can become an attribute trigger.
type AttributeKey struct {
	Key   string
	Label string
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
