package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
)

// Noop is a local/CI processor. Checkout/Portal are unavailable; webhooks accept a simple JSON envelope.
type Noop struct{}

func NewNoop() *Noop { return &Noop{} }

func (n *Noop) Name() string { return "noop" }

func (n *Noop) EnsureCustomer(_ context.Context, in CustomerInput) (CustomerRef, error) {
	if ref := strings.TrimSpace(in.ExistingCustomerRef); ref != "" {
		return CustomerRef{Ref: ref}, nil
	}
	org := strings.TrimSpace(in.OrganisationID)
	if org == "" {
		return CustomerRef{}, apperr.Validation("organisation_id is required")
	}
	return CustomerRef{Ref: "cus_noop_" + org}, nil
}

func (n *Noop) CreateCheckout(context.Context, CheckoutInput) (CheckoutSession, error) {
	return CheckoutSession{}, apperr.Validation("billing checkout unavailable (PAYMENTS_PROVIDER=noop)")
}

func (n *Noop) CreatePortal(context.Context, PortalInput) (PortalSession, error) {
	return PortalSession{}, apperr.Validation("billing portal unavailable (PAYMENTS_PROVIDER=noop)")
}

// ParseWebhook accepts a KYC test envelope (not Stripe-signed):
//
//	{
//	  "id": "evt_…",
//	  "type": "customer.subscription.updated",
//	  "organisation_id": "…",
//	  "customer_ref": "…",
//	  "subscription_ref": "…",
//	  "price_ref": "…",
//	  "plan_id": "…",
//	  "status": "active",
//	  "current_period_end": "2026-01-01T00:00:00Z"
//	}
func (n *Noop) ParseWebhook(_ http.Header, body []byte) (Event, error) {
	var raw struct {
		ID               string  `json:"id"`
		Type             string  `json:"type"`
		OrganisationID   string  `json:"organisation_id"`
		CustomerRef      string  `json:"customer_ref"`
		SubscriptionRef  string  `json:"subscription_ref"`
		PriceRef         string  `json:"price_ref"`
		PlanID           string  `json:"plan_id"`
		Status           string  `json:"status"`
		CurrentPeriodEnd *string `json:"current_period_end"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Event{}, apperr.Validation("invalid webhook JSON")
	}
	if strings.TrimSpace(raw.ID) == "" {
		return Event{}, apperr.Validation("webhook id is required")
	}
	ev := Event{
		Ref:             strings.TrimSpace(raw.ID),
		Type:            strings.TrimSpace(raw.Type),
		OrganisationID:  strings.TrimSpace(raw.OrganisationID),
		CustomerRef:     strings.TrimSpace(raw.CustomerRef),
		SubscriptionRef: strings.TrimSpace(raw.SubscriptionRef),
		PriceRef:        strings.TrimSpace(raw.PriceRef),
		PlanID:          strings.TrimSpace(raw.PlanID),
		Status:          strings.TrimSpace(raw.Status),
		Raw:             json.RawMessage(append([]byte(nil), body...)),
	}
	if raw.CurrentPeriodEnd != nil && strings.TrimSpace(*raw.CurrentPeriodEnd) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw.CurrentPeriodEnd))
		if err != nil {
			return Event{}, apperr.Validation(fmt.Sprintf("invalid current_period_end: %v", err))
		}
		ev.CurrentPeriodEnd = &t
	}
	return ev, nil
}
