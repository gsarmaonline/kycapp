package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/payments"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SetPayments configures the PSP executor and default Checkout return URLs.
func (s *Service) SetPayments(p payments.Processor, successURL, cancelURL, appOrigin string) {
	s.payments = p
	s.checkoutSuccessURL = strings.TrimSpace(successURL)
	s.checkoutCancelURL = strings.TrimSpace(cancelURL)
	if origin := strings.TrimRight(strings.TrimSpace(appOrigin), "/"); origin != "" {
		s.publicBaseURL = origin
	}
}

func (s *Service) PaymentsProvider() string {
	return s.paymentsOrNoop().Name()
}

func (s *Service) paymentsOrNoop() payments.Processor {
	if s.payments != nil {
		return s.payments
	}
	return payments.NewNoop()
}

type UpsertPlanPriceInput struct {
	Interval          string
	Currency          string
	UnitAmount        int64
	ProcessorPriceRef string
	Status            string
}

func (s *Service) UpsertPlanPrice(ctx context.Context, planID string, in UpsertPlanPriceInput) (sqlc.PlanPrice, error) {
	if _, err := s.db.Q().GetPlan(ctx, planID); err != nil {
		return sqlc.PlanPrice{}, mapNotFound(err, "plan not found")
	}
	interval := strings.TrimSpace(in.Interval)
	if interval == "" {
		interval = "month"
	}
	switch interval {
	case "month", "year":
	default:
		return sqlc.PlanPrice{}, apperr.Validation("interval must be month or year")
	}
	currency := strings.ToLower(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "usd"
	}
	priceRef := strings.TrimSpace(in.ProcessorPriceRef)
	if priceRef == "" {
		return sqlc.PlanPrice{}, apperr.Validation("processor_price_ref is required")
	}
	if in.UnitAmount < 0 {
		return sqlc.PlanPrice{}, apperr.Validation("unit_amount must be >= 0")
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "archived":
	default:
		return sqlc.PlanPrice{}, apperr.Validation("status must be active or archived")
	}
	proc := s.paymentsOrNoop().Name()
	row, err := s.db.Q().UpsertPlanPrice(ctx, sqlc.UpsertPlanPriceParams{
		ID:                ids.New(),
		PlanID:            planID,
		Interval:          interval,
		Currency:          currency,
		UnitAmount:        in.UnitAmount,
		Processor:         proc,
		ProcessorPriceRef: priceRef,
		Status:            status,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.PlanPrice{}, apperr.Conflict("processor_price_ref already linked")
		}
		return sqlc.PlanPrice{}, err
	}
	return row, nil
}

func (s *Service) ListPlanPrices(ctx context.Context, planID string) ([]sqlc.PlanPrice, error) {
	if _, err := s.db.Q().GetPlan(ctx, planID); err != nil {
		return nil, mapNotFound(err, "plan not found")
	}
	return s.db.Q().ListPlanPricesByPlan(ctx, planID)
}

type CreateCheckoutInput struct {
	PlanID     string
	Interval   string
	SuccessURL string
	CancelURL  string
}

type CheckoutResult struct {
	URL string
}

func (s *Service) CreateBillingCheckout(ctx context.Context, orgID string, in CreateCheckoutInput) (CheckoutResult, error) {
	p, err := RequireUser(ctx)
	if err != nil {
		return CheckoutResult{}, err
	}
	org, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return CheckoutResult{}, err
	}
	planID := strings.TrimSpace(in.PlanID)
	if planID == "" {
		return CheckoutResult{}, apperr.Validation("plan_id is required")
	}
	if _, err := s.db.Q().GetPlan(ctx, planID); err != nil {
		return CheckoutResult{}, mapNotFound(err, "plan not found")
	}
	interval := strings.TrimSpace(in.Interval)
	if interval == "" {
		interval = "month"
	}
	proc := s.paymentsOrNoop()
	price, err := s.db.Q().GetActivePlanPrice(ctx, sqlc.GetActivePlanPriceParams{
		PlanID:    planID,
		Interval:  interval,
		Processor: proc.Name(),
	})
	if err != nil {
		return CheckoutResult{}, mapNotFound(err, "plan has no active price for this interval")
	}

	user, err := s.GetUser(ctx, p.UserID)
	if err != nil {
		return CheckoutResult{}, err
	}
	existingRef := ""
	if bc, err := s.db.Q().GetBillingCustomerByOrganisation(ctx, orgID); err == nil {
		existingRef = bc.CustomerRef
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CheckoutResult{}, err
	}

	cus, err := proc.EnsureCustomer(ctx, payments.CustomerInput{
		OrganisationID:      orgID,
		Email:               user.Email,
		Name:                org.Name,
		ExistingCustomerRef: existingRef,
	})
	if err != nil {
		return CheckoutResult{}, err
	}
	if _, err := s.db.Q().UpsertBillingCustomer(ctx, sqlc.UpsertBillingCustomerParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Processor:      proc.Name(),
		CustomerRef:    cus.Ref,
		Email:          user.Email,
	}); err != nil {
		return CheckoutResult{}, err
	}

	successURL := firstNonEmpty(in.SuccessURL, s.checkoutSuccessURL, s.publicBaseURL+"/orgs/"+orgID+"/billing?checkout=success")
	cancelURL := firstNonEmpty(in.CancelURL, s.checkoutCancelURL, s.publicBaseURL+"/orgs/"+orgID+"/billing?checkout=cancel")

	sess, err := proc.CreateCheckout(ctx, payments.CheckoutInput{
		CustomerRef:    cus.Ref,
		PriceRef:       price.ProcessorPriceRef,
		OrganisationID: orgID,
		PlanID:         planID,
		SuccessURL:     successURL,
		CancelURL:      cancelURL,
	})
	if err != nil {
		return CheckoutResult{}, err
	}
	return CheckoutResult{URL: sess.URL}, nil
}

type CreatePortalInput struct {
	ReturnURL string
}

type PortalResult struct {
	URL string
}

func (s *Service) CreateBillingPortal(ctx context.Context, orgID string, in CreatePortalInput) (PortalResult, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return PortalResult{}, err
	}
	bc, err := s.db.Q().GetBillingCustomerByOrganisation(ctx, orgID)
	if err != nil {
		return PortalResult{}, mapNotFound(err, "no billing customer; complete checkout first")
	}
	proc := s.paymentsOrNoop()
	returnURL := firstNonEmpty(in.ReturnURL, s.publicBaseURL+"/orgs/"+orgID+"/billing")
	sess, err := proc.CreatePortal(ctx, payments.PortalInput{
		CustomerRef: bc.CustomerRef,
		ReturnURL:   returnURL,
	})
	if err != nil {
		return PortalResult{}, err
	}
	return PortalResult{URL: sess.URL}, nil
}

// HandlePaymentWebhook verifies, idempotently stores, and reconciles a processor event.
func (s *Service) HandlePaymentWebhook(ctx context.Context, headers http.Header, body []byte) error {
	proc := s.paymentsOrNoop()
	ev, err := proc.ParseWebhook(headers, body)
	if err != nil {
		return err
	}
	payload := ev.Raw
	if len(payload) == 0 {
		payload = json.RawMessage([]byte("{}"))
	}
	_, err = s.db.Q().InsertProcessorEvent(ctx, sqlc.InsertProcessorEventParams{
		ID:        ids.New(),
		Processor: proc.Name(),
		EventRef:  ev.Ref,
		EventType: ev.Type,
		Payload:   payload,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	existing, err := s.db.Q().GetProcessorEvent(ctx, sqlc.GetProcessorEventParams{
		Processor: proc.Name(),
		EventRef:  ev.Ref,
	})
	if err != nil {
		return err
	}
	if existing.ProcessedAt.Valid {
		return nil
	}

	if err := s.applyPaymentEvent(ctx, proc.Name(), ev); err != nil {
		return err
	}
	return s.db.Q().MarkProcessorEventProcessed(ctx, sqlc.MarkProcessorEventProcessedParams{
		Processor: proc.Name(),
		EventRef:  ev.Ref,
	})
}

func (s *Service) applyPaymentEvent(ctx context.Context, processor string, ev payments.Event) error {
	orgID := strings.TrimSpace(ev.OrganisationID)
	if orgID == "" && ev.CustomerRef != "" {
		if bc, err := s.db.Q().GetBillingCustomerByProcessorRef(ctx, sqlc.GetBillingCustomerByProcessorRefParams{
			Processor:   processor,
			CustomerRef: ev.CustomerRef,
		}); err == nil {
			orgID = bc.OrganisationID
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if orgID == "" && ev.SubscriptionRef != "" {
		if sub, err := s.db.Q().GetSubscriptionByProcessorRef(ctx, sqlc.GetSubscriptionByProcessorRefParams{
			Processor:       pgtype.Text{String: processor, Valid: true},
			SubscriptionRef: pgtype.Text{String: ev.SubscriptionRef, Valid: true},
		}); err == nil {
			orgID = sub.OrganisationID
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if orgID == "" {
		// Unknown / irrelevant event — acknowledge without domain change.
		return nil
	}
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return err
	}

	if ev.CustomerRef != "" {
		email := ""
		if bc, err := s.db.Q().GetBillingCustomerByOrganisation(ctx, orgID); err == nil {
			email = bc.Email
		}
		if _, err := s.db.Q().UpsertBillingCustomer(ctx, sqlc.UpsertBillingCustomerParams{
			ID:             ids.New(),
			OrganisationID: orgID,
			Processor:      processor,
			CustomerRef:    ev.CustomerRef,
			Email:          email,
		}); err != nil {
			return err
		}
	}

	status := strings.TrimSpace(ev.Status)
	if status == "" && ev.SubscriptionRef == "" {
		return nil
	}
	if status == "" {
		status = "active"
	}
	switch status {
	case "trialing", "active", "past_due", "canceled":
	default:
		return apperr.Validation("invalid subscription status from processor")
	}

	planID := strings.TrimSpace(ev.PlanID)
	if planID == "" && ev.PriceRef != "" {
		if price, err := s.db.Q().GetPlanPriceByProcessorRef(ctx, sqlc.GetPlanPriceByProcessorRefParams{
			Processor:         processor,
			ProcessorPriceRef: ev.PriceRef,
		}); err == nil {
			planID = price.PlanID
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if planID == "" {
		if sub, err := s.db.Q().GetSubscriptionByOrganisation(ctx, orgID); err == nil {
			planID = sub.PlanID
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if planID == "" {
		return apperr.Validation("cannot reconcile subscription without plan_id or price_ref")
	}
	if _, err := s.db.Q().GetPlan(ctx, planID); err != nil {
		return mapNotFound(err, "plan not found")
	}

	var periodEnd pgtype.Timestamptz
	if ev.CurrentPeriodEnd != nil {
		periodEnd = pgtype.Timestamptz{Time: ev.CurrentPeriodEnd.UTC(), Valid: true}
	}

	var subRef pgtype.Text
	if ev.SubscriptionRef != "" {
		subRef = pgtype.Text{String: ev.SubscriptionRef, Valid: true}
	}
	_, err := s.db.Q().UpsertSubscriptionFromProcessor(ctx, sqlc.UpsertSubscriptionFromProcessorParams{
		ID:               ids.New(),
		OrganisationID:   orgID,
		PlanID:           planID,
		Status:           status,
		CurrentPeriodEnd: periodEnd,
		Processor:        pgtype.Text{String: processor, Valid: true},
		SubscriptionRef:  subRef,
	})
	return err
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
