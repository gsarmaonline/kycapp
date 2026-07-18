package jobs

import "encoding/json"

// AutomationEventArgs is enqueued when a domain event may trigger automations.
type AutomationEventArgs struct {
	OrganisationID string          `json:"organisation_id"`
	Trigger        string          `json:"trigger"`
	Payload        json.RawMessage `json:"payload"`
}

func (AutomationEventArgs) Kind() string { return "automation_event" }
