package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Processor executes PSP APIs (Stripe first). KYC remains source of truth for access.
type Processor interface {
	Name() string
	EnsureCustomer(ctx context.Context, in CustomerInput) (CustomerRef, error)
	CreateCheckout(ctx context.Context, in CheckoutInput) (CheckoutSession, error)
	CreatePortal(ctx context.Context, in PortalInput) (PortalSession, error)
	ParseWebhook(headers http.Header, body []byte) (Event, error)
}

type CustomerInput struct {
	OrganisationID      string
	Email               string
	Name                string
	ExistingCustomerRef string
}

type CustomerRef struct {
	Ref string
}

type CheckoutInput struct {
	CustomerRef    string
	PriceRef       string
	OrganisationID string
	PlanID         string
	SuccessURL     string
	CancelURL      string
}

type CheckoutSession struct {
	URL        string
	SessionRef string
}

type PortalInput struct {
	CustomerRef string
	ReturnURL   string
}

type PortalSession struct {
	URL string
}

// Event is a verified, normalized processor webhook outcome.
type Event struct {
	Ref              string
	Type             string
	OrganisationID   string
	CustomerRef      string
	SubscriptionRef  string
	PriceRef         string
	PlanID           string
	Status           string // trialing|active|past_due|canceled (empty if n/a)
	CurrentPeriodEnd *time.Time
	Raw              json.RawMessage
}
