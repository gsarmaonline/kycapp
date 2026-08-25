package service

import (
	"context"
	"strings"

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

// CapabilityTemplate is one named starter set: a vocabulary and the shape that
// usually goes with it.
//
// Roles are the half worth having. Almost every product has admin extending
// member extending viewer, and a template that stopped at capabilities left
// every merchant building that chain by hand. The shape is the part that
// generalises; what each role grants is the part that does not, which is why
// it is offered rather than seeded.
type CapabilityTemplate struct {
	Key         string                   `json:"key"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Items       []CapabilityTemplateItem `json:"items"`
	// Roles are created in order, so a role may only extend one named before
	// it. That keeps the chain resolvable in a single pass and makes a cycle
	// unwritable rather than merely rejected.
	Roles []CapabilityTemplateRole `json:"roles,omitempty"`
}

// CapabilityTemplateRole is one role a template would create.
//
// A template never creates a grant. A role confers nothing until one carries
// it, so this is a starting point a merchant edits, while issuing a grant would
// be granting access nobody authorised.
type CapabilityTemplateRole struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Capabilities this role adds beyond its parents.
	Capabilities []string `json:"capabilities"`
	// Extends names roles earlier in the same template.
	Extends []string `json:"extends,omitempty"`
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
		Roles: []CapabilityTemplateRole{
			{
				Key: "viewer", Name: "Viewer",
				Description:  "Can see the account but change nothing.",
				Capabilities: []string{"profile:read", "members:read", "billing:read"},
			},
			{
				Key: "member", Name: "Member",
				Description:  "A viewer who can also edit their own profile.",
				Capabilities: []string{"profile:write"},
				Extends:      []string{"viewer"},
			},
			{
				Key: "admin", Name: "Admin",
				Description:  "A member who can also manage the team and billing.",
				Capabilities: []string{"members:invite", "members:remove", "billing:manage"},
				Extends:      []string{"member"},
			},
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
		Roles: []CapabilityTemplateRole{
			{
				Key: "reader", Name: "Reader",
				Description:  "Can open documents but not change them.",
				Capabilities: []string{"document:read", "workspace:read"},
			},
			{
				Key: "editor", Name: "Editor",
				Description:  "A reader who can also write and share.",
				Capabilities: []string{"document:write", "document:share"},
				Extends:      []string{"reader"},
			},
			{
				Key: "owner", Name: "Owner",
				Description:  "An editor who can also delete and configure the workspace.",
				Capabilities: []string{"document:delete", "workspace:manage"},
				Extends:      []string{"editor"},
			},
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
		resource, action, _ := strings.Cut(item.Key, ":")
		if err := s.db.Q().CreateAppCapabilityFromTemplate(ctx, sqlc.CreateAppCapabilityFromTemplateParams{
			ID:             ids.New(),
			OrganisationID: orgID,
			Resource:       resource,
			Action:         action,
			Description:    item.Description,
			Source:         tpl.Source(),
		}); err != nil {
			return nil, err
		}
	}
	if err := s.applyTemplateRoles(ctx, orgID, tpl); err != nil {
		return nil, err
	}
	// No grant is created here, and none ever should be. A role confers nothing
	// until a grant carries it, which is exactly why seeding roles is a
	// starting point and seeding a grant would be issuing access nobody asked
	// for. TestTemplatesNeverIssueAGrant holds that line.
	return s.db.Q().ListAppCapabilities(ctx, orgID)
}

// applyTemplateRoles creates the template's roles and the inheritance between
// them.
//
// Roles are created in template order, so a parent always exists before the
// child naming it. That is what makes the chain resolvable in one pass, and it
// makes a cycle unwritable rather than merely rejected afterwards.
func (s *Service) applyTemplateRoles(ctx context.Context, orgID string, tpl CapabilityTemplate) error {
	if len(tpl.Roles) == 0 {
		return nil
	}
	// Key to id, for resolving Extends. Populated as roles are created, and
	// from the store for a key that already existed, so re-applying converges
	// on the same chain rather than skipping it.
	byKey := map[string]string{}

	for _, role := range tpl.Roles {
		own := role.Capabilities
		if own == nil {
			own = []string{}
		}
		if err := s.db.Q().CreateAppRoleFromTemplate(ctx, sqlc.CreateAppRoleFromTemplateParams{
			ID:              ids.New(),
			OrganisationID:  orgID,
			Key:             role.Key,
			Name:            role.Name,
			Description:     role.Description,
			OwnCapabilities: own,
			Source:          tpl.Source(),
		}); err != nil {
			return err
		}
		row, err := s.db.Q().GetAppRoleByKey(ctx, sqlc.GetAppRoleByKeyParams{
			OrganisationID: orgID, Key: role.Key,
		})
		if err != nil {
			return err
		}
		byKey[role.Key] = row.ID
	}

	for _, role := range tpl.Roles {
		if len(role.Extends) == 0 {
			continue
		}
		parents := make([]string, 0, len(role.Extends))
		for _, key := range role.Extends {
			id, ok := byKey[key]
			if !ok {
				// Only reachable from a malformed template, since Extends may
				// only name a role earlier in the same one.
				return apperr.Validation("template " + tpl.Key + " extends unknown role " + key)
			}
			parents = append(parents, id)
		}
		if err := s.setAppRoleParents(ctx, orgID, byKey[role.Key], parents); err != nil {
			return err
		}
	}
	// One recompute at the end rather than per role: inheritance is resolved
	// wholesale, so doing it once the chain is complete is both cheaper and the
	// only point at which it is correct.
	return s.recomputeAppRoles(ctx, orgID)
}
