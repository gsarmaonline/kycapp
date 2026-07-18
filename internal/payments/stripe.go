package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Stripe executes Checkout, Portal, Customers, and signed webhooks.
type Stripe struct {
	secretKey     string
	webhookSecret string
}

func NewStripe(secretKey, webhookSecret string) (*Stripe, error) {
	if strings.TrimSpace(secretKey) == "" {
		return nil, fmt.Errorf("STRIPE_SECRET_KEY is required when PAYMENTS_PROVIDER=stripe")
	}
	return &Stripe{
		secretKey:     strings.TrimSpace(secretKey),
		webhookSecret: strings.TrimSpace(webhookSecret),
	}, nil
}

func (s *Stripe) Name() string { return "stripe" }

func (s *Stripe) EnsureCustomer(ctx context.Context, in CustomerInput) (CustomerRef, error) {
	stripe.Key = s.secretKey
	if ref := strings.TrimSpace(in.ExistingCustomerRef); ref != "" {
		return CustomerRef{Ref: ref}, nil
	}
	params := &stripe.CustomerParams{
		Email: stripe.String(strings.TrimSpace(in.Email)),
		Name:  stripe.String(strings.TrimSpace(in.Name)),
	}
	params.AddMetadata("org_id", strings.TrimSpace(in.OrganisationID))
	params.Context = ctx
	cus, err := customer.New(params)
	if err != nil {
		return CustomerRef{}, fmt.Errorf("stripe create customer: %w", err)
	}
	return CustomerRef{Ref: cus.ID}, nil
}

func (s *Stripe) CreateCheckout(ctx context.Context, in CheckoutInput) (CheckoutSession, error) {
	stripe.Key = s.secretKey
	if strings.TrimSpace(in.CustomerRef) == "" || strings.TrimSpace(in.PriceRef) == "" {
		return CheckoutSession{}, apperr.Validation("customer and price are required")
	}
	if strings.TrimSpace(in.SuccessURL) == "" || strings.TrimSpace(in.CancelURL) == "" {
		return CheckoutSession{}, apperr.Validation("success_url and cancel_url are required")
	}
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:   stripe.String(in.CustomerRef),
		SuccessURL: stripe.String(in.SuccessURL),
		CancelURL:  stripe.String(in.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(in.PriceRef),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{},
	}
	params.AddMetadata("org_id", in.OrganisationID)
	params.AddMetadata("plan_id", in.PlanID)
	params.SubscriptionData.AddMetadata("org_id", in.OrganisationID)
	params.SubscriptionData.AddMetadata("plan_id", in.PlanID)
	params.Context = ctx
	sess, err := checkoutsession.New(params)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("stripe checkout: %w", err)
	}
	return CheckoutSession{URL: sess.URL, SessionRef: sess.ID}, nil
}

func (s *Stripe) CreatePortal(ctx context.Context, in PortalInput) (PortalSession, error) {
	stripe.Key = s.secretKey
	if strings.TrimSpace(in.CustomerRef) == "" {
		return PortalSession{}, apperr.Validation("customer is required")
	}
	if strings.TrimSpace(in.ReturnURL) == "" {
		return PortalSession{}, apperr.Validation("return_url is required")
	}
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(in.CustomerRef),
		ReturnURL: stripe.String(in.ReturnURL),
	}
	params.Context = ctx
	sess, err := session.New(params)
	if err != nil {
		return PortalSession{}, fmt.Errorf("stripe portal: %w", err)
	}
	return PortalSession{URL: sess.URL}, nil
}

func (s *Stripe) ParseWebhook(headers http.Header, body []byte) (Event, error) {
	if s.webhookSecret == "" {
		return Event{}, apperr.Validation("STRIPE_WEBHOOK_SECRET is not configured")
	}
	sig := headers.Get("Stripe-Signature")
	if sig == "" {
		return Event{}, apperr.Unauthorized("missing Stripe-Signature")
	}
	stripeEvent, err := webhook.ConstructEvent(body, sig, s.webhookSecret)
	if err != nil {
		return Event{}, apperr.Unauthorized("invalid Stripe webhook signature")
	}
	ev := Event{
		Ref:  stripeEvent.ID,
		Type: string(stripeEvent.Type),
		Raw:  json.RawMessage(append([]byte(nil), body...)),
	}
	switch stripeEvent.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(stripeEvent.Data.Raw, &sess); err != nil {
			return Event{}, fmt.Errorf("decode checkout.session: %w", err)
		}
		ev.OrganisationID = meta(sess.Metadata, "org_id")
		ev.PlanID = meta(sess.Metadata, "plan_id")
		if sess.Customer != nil {
			ev.CustomerRef = sess.Customer.ID
		}
		if sess.Subscription != nil {
			ev.SubscriptionRef = sess.Subscription.ID
		}
		ev.Status = "active"
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(stripeEvent.Data.Raw, &sub); err != nil {
			return Event{}, fmt.Errorf("decode subscription: %w", err)
		}
		ev.OrganisationID = meta(sub.Metadata, "org_id")
		ev.PlanID = meta(sub.Metadata, "plan_id")
		ev.SubscriptionRef = sub.ID
		if sub.Customer != nil {
			ev.CustomerRef = sub.Customer.ID
		}
		if sub.Items != nil && len(sub.Items.Data) > 0 {
			item := sub.Items.Data[0]
			if item.Price != nil {
				ev.PriceRef = item.Price.ID
			}
			if item.CurrentPeriodEnd > 0 {
				t := time.Unix(item.CurrentPeriodEnd, 0).UTC()
				ev.CurrentPeriodEnd = &t
			}
		}
		ev.Status = mapStripeSubStatus(string(sub.Status), stripeEvent.Type == "customer.subscription.deleted")
	case "invoice.paid", "invoice.payment_failed":
		var inv stripe.Invoice
		if err := json.Unmarshal(stripeEvent.Data.Raw, &inv); err != nil {
			return Event{}, fmt.Errorf("decode invoice: %w", err)
		}
		if inv.Customer != nil {
			ev.CustomerRef = inv.Customer.ID
		}
		if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil && inv.Parent.SubscriptionDetails.Subscription != nil {
			ev.SubscriptionRef = inv.Parent.SubscriptionDetails.Subscription.ID
		}
		if stripeEvent.Type == "invoice.paid" {
			ev.Status = "active"
		} else {
			ev.Status = "past_due"
		}
		if inv.Lines != nil {
			for _, line := range inv.Lines.Data {
				if line.Pricing != nil && line.Pricing.PriceDetails != nil && line.Pricing.PriceDetails.Price != "" {
					ev.PriceRef = line.Pricing.PriceDetails.Price
					break
				}
			}
		}
	default:
		// Acknowledge unknown types without applying domain changes.
	}
	return ev, nil
}

func meta(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[key])
}

func mapStripeSubStatus(status string, deleted bool) string {
	if deleted {
		return "canceled"
	}
	switch status {
	case "trialing":
		return "trialing"
	case "active":
		return "active"
	case "past_due", "unpaid":
		return "past_due"
	case "canceled", "incomplete_expired":
		return "canceled"
	default:
		return "active"
	}
}
