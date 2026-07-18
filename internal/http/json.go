package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		status := http.StatusBadRequest
		switch {
		case errors.Is(ae.Err, apperr.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(ae.Err, apperr.ErrConflict), errors.Is(ae.Err, apperr.ErrIdempotencyConflict):
			status = http.StatusConflict
		case errors.Is(ae.Err, apperr.ErrValidation):
			status = http.StatusBadRequest
		case errors.Is(ae.Err, apperr.ErrUnauthorized):
			status = http.StatusUnauthorized
		case errors.Is(ae.Err, apperr.ErrForbidden):
			status = http.StatusForbidden
		case errors.Is(ae.Err, apperr.ErrRateLimited):
			status = http.StatusTooManyRequests
		}
		writeJSON(w, status, map[string]any{
			"error": map[string]string{"code": ae.Code, "message": ae.Message},
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error": map[string]string{"code": "internal_error", "message": "internal server error"},
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func queryLimit(r *http.Request) int32 {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return 50
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 50
	}
	return int32(n)
}

func orgJSON(o sqlc.Organisation) map[string]any {
	return map[string]any{
		"id":             o.ID,
		"name":           o.Name,
		"slug":           o.Slug,
		"status":         o.Status,
		"logo_url":       o.LogoUrl,
		"primary_color":  o.PrimaryColor,
		"accent_color":   o.AccentColor,
		"email_footer":   o.EmailFooter,
		"email_font":     o.EmailFont,
		"created_at":     o.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":     o.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func userJSON(u sqlc.User) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"email":          u.Email,
		"name":           u.Name,
		"status":         u.Status,
		"platform_admin": u.PlatformAdmin,
		"created_at":     u.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":     u.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func membershipJSON(m sqlc.Membership) map[string]any {
	return map[string]any{
		"id":              m.ID,
		"organisation_id": m.OrganisationID,
		"user_id":         m.UserID,
		"role_id":         m.RoleID,
		"status":          m.Status,
		"created_at":      m.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func subscriptionJSON(sub sqlc.Subscription) map[string]any {
	out := map[string]any{
		"id":              sub.ID,
		"organisation_id": sub.OrganisationID,
		"plan_id":         sub.PlanID,
		"status":          sub.Status,
	}
	if sub.CurrentPeriodEnd.Valid {
		out["current_period_end"] = sub.CurrentPeriodEnd.Time.UTC().Format(time.RFC3339Nano)
	} else {
		out["current_period_end"] = nil
	}
	if sub.Processor.Valid {
		out["processor"] = sub.Processor.String
	}
	if sub.SubscriptionRef.Valid {
		out["subscription_ref"] = sub.SubscriptionRef.String
	}
	return out
}

func planPriceJSON(p sqlc.PlanPrice) map[string]any {
	return map[string]any{
		"id":                  p.ID,
		"plan_id":             p.PlanID,
		"interval":            p.Interval,
		"currency":            p.Currency,
		"unit_amount":         p.UnitAmount,
		"processor":           p.Processor,
		"processor_price_ref": p.ProcessorPriceRef,
		"status":              p.Status,
	}
}

func permissionJSON(p sqlc.Permission) map[string]any {
	return map[string]any{
		"id":          p.ID,
		"key":         p.Key,
		"resource":    p.Resource,
		"action":      p.Action,
		"category":    p.Category,
		"description": p.Description,
		"is_system":   p.IsSystem,
	}
}

func roleJSON(role service.RoleView) map[string]any {
	return map[string]any{
		"id":              role.Role.ID,
		"organisation_id": role.Role.OrganisationID,
		"key":             role.Role.Key,
		"name":            role.Role.Name,
		"description":     role.Role.Description,
		"is_system":       role.Role.IsSystem,
		"permission_keys": role.PermissionKeys,
	}
}

func planJSON(p service.PlanView) map[string]any {
	return map[string]any{
		"id":               p.Plan.ID,
		"key":              p.Plan.Key,
		"name":             p.Plan.Name,
		"status":           p.Plan.Status,
		"entitlement_keys": p.EntitlementKeys,
	}
}

func entitlementJSON(e sqlc.Entitlement) map[string]any {
	return map[string]any{
		"id":          e.ID,
		"key":         e.Key,
		"description": e.Description,
	}
}

func authResultJSON(a service.AuthResult) map[string]any {
	return map[string]any{
		"token":      a.Token,
		"expires_at": a.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"user":       userJSON(a.User),
	}
}

func attributeDefinitionJSON(d sqlc.AttributeDefinition) map[string]any {
	var enumValues []string
	if len(d.EnumValues) > 0 {
		_ = json.Unmarshal(d.EnumValues, &enumValues)
	}
	if enumValues == nil {
		enumValues = []string{}
	}
	return map[string]any{
		"id":              d.ID,
		"organisation_id": d.OrganisationID,
		"key":             d.Key,
		"label":           d.Label,
		"description":     d.Description,
		"value_type":      d.ValueType,
		"section":         d.Section,
		"sort_order":      d.SortOrder,
		"required":        d.Required,
		"enum_values":     enumValues,
		"is_pii":          d.IsPii,
		"is_system":       d.IsSystem,
		"status":          d.Status,
		"created_at":      d.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      d.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func appUserJSON(u sqlc.AppUser) map[string]any {
	attrs := map[string]any{}
	if len(u.Attributes) > 0 {
		_ = json.Unmarshal(u.Attributes, &attrs)
	}
	out := map[string]any{
		"id":              u.ID,
		"organisation_id": u.OrganisationID,
		"display_name":    u.DisplayName,
		"status":          u.Status,
		"attributes":      attrs,
		"created_at":      u.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      u.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if u.ExternalID.Valid {
		out["external_id"] = u.ExternalID.String
	} else {
		out["external_id"] = nil
	}
	if u.Email.Valid {
		out["email"] = u.Email.String
	} else {
		out["email"] = nil
	}
	return out
}

func emailTemplateJSON(t sqlc.EmailTemplate) map[string]any {
	return map[string]any{
		"id":              t.ID,
		"organisation_id": t.OrganisationID,
		"key":             t.Key,
		"name":            t.Name,
		"description":     t.Description,
		"subject":         t.Subject,
		"body_text":       t.BodyText,
		"body_html":       t.BodyHtml,
		"status":          t.Status,
		"is_system":       t.IsSystem,
		"created_at":      t.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      t.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
