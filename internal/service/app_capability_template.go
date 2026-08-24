package service

import (
	"context"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// Starter templates for a merchant's capability vocabulary.
//
// Nothing is seeded into customer access on signup, and that is the right call:
// the vocabulary is deliberately open, so a seeded default is a guess about
// somebody else's product. A wrong guess is worse than an empty list, because
// merchants work around it rather than delete it and then it has to be
// supported for ever.
//
// The cold start is a real problem all the same. Issuing one grant requires a
// scope kind, then a capability, then a role, three objects deep before
// anything happens, and every one of those pages opens empty.
//
// A template is the answer that is not a default. Same rows in the database,
// entirely different thing: applied by an explicit click, on a page the
// merchant is already looking at, and recorded as template-sourced so the
// answer to "why does this exist?" survives the person who clicked it.

// CapabilityTemplate is one named starter set.
type CapabilityTemplate struct {
	Key         string                   `json:"key"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Items       []CapabilityTemplateItem `json:"items"`
}

// CapabilityTemplateItem is one capability a template would declare.
type CapabilityTemplateItem struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Source is what lands in app_capabilities.source. Prefixed, so a reader can
// tell template provenance from any other source added later.
func (t CapabilityTemplate) Source() string { return "template:" + t.Key }

// CapabilityTemplates is the closed set, defined in code rather than seeded per
// organisation. A merchant applies one, edits the result, and owns it from then
// on: the template is a starting point, never a thing KYC keeps in step.
var CapabilityTemplates = []CapabilityTemplate{
	{
		Key:         "team_accounts",
		Name:        "Team accounts",
		Description: "Customers belong to an account and manage their own team, profile and billing.",
		Items: []CapabilityTemplateItem{
			{Key: "profile:read", Description: "See a profile"},
			{Key: "profile:write", Description: "Edit a profile"},
			{Key: "members:read", Description: "See who is on the account"},
			{Key: "members:invite", Description: "Invite someone to the account"},
			{Key: "members:remove", Description: "Remove someone from the account"},
			{Key: "billing:read", Description: "See invoices and plan"},
			{Key: "billing:manage", Description: "Change plan or payment method"},
		},
	},
	{
		Key:         "content_workspace",
		Name:        "Content workspace",
		Description: "Customers create and share documents inside a workspace.",
		Items: []CapabilityTemplateItem{
			{Key: "document:read", Description: "Open a document"},
			{Key: "document:write", Description: "Edit a document"},
			{Key: "document:delete", Description: "Delete a document"},
			{Key: "document:share", Description: "Share a document with someone else"},
			{Key: "workspace:read", Description: "See the workspace"},
			{Key: "workspace:manage", Description: "Rename or configure the workspace"},
		},
	},
}

// CapabilityTemplateByKey finds one template.
func CapabilityTemplateByKey(key string) (CapabilityTemplate, bool) {
	for _, t := range CapabilityTemplates {
		if t.Key == key {
			return t, true
		}
	}
	return CapabilityTemplate{}, false
}

// ApplyCapabilityTemplate declares a template's capabilities for one
// organisation and returns the resulting list.
//
// Applying twice is harmless and never relabels: a key that already exists is
// already declared, whoever declared it, so an authored capability keeps its
// authored provenance even if a template later names the same key.
func (s *Service) ApplyCapabilityTemplate(ctx context.Context, orgID, key string) ([]sqlc.AppCapability, error) {
	tpl, ok := CapabilityTemplateByKey(key)
	if !ok {
		return nil, apperr.Validation("unknown capability template: " + key)
	}
	for _, item := range tpl.Items {
		// Every key goes through the same validation an authored one does. A
		// template that could write a key the registry would reject would be a
		// way around the vocabulary rules rather than a shortcut through them.
		if err := ValidAppCapabilityKey(item.Key); err != nil {
			return nil, apperr.Validation(err.Error())
		}
		if err := s.db.Q().CreateAppCapabilityFromTemplate(ctx, sqlc.CreateAppCapabilityFromTemplateParams{
			ID:             ids.New(),
			OrganisationID: orgID,
			Key:            item.Key,
			Description:    item.Description,
			Source:         tpl.Source(),
		}); err != nil {
			return nil, err
		}
	}
	return s.db.Q().ListAppCapabilities(ctx, orgID)
}
