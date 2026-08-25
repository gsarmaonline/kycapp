package service

import (
	"context"
	"sort"

	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// What exists, beside the model that says what may exist.
//
// The map draws a schema, and access_explain.go states why: owner, admin and
// member are rows in a table, not types, so they cannot appear in a picture of
// types. The rule it drew from that was "the model in one place, the instances
// in another". This is the same rule with the second place moved onto the
// canvas, because a merchant asking "where are my roles?" is looking at the
// picture when they ask it.
//
// What keeps that safe is the cap, not the type. There is no line between a set
// worth drawing and a set that is not: roles number in the tens and documents in
// the millions, and both are instances of a declared type. So every type is
// drawn, none is drawn past MerchantInstanceCap, and a type over the cap says so
// rather than showing a hundred rows that read as all of them.

// MerchantInstanceCap bounds what one type contributes to the picture.
//
// A hundred is past the point where a reader counts nodes and short of the
// point where the canvas stops being a picture. It is not a page size: there is
// no second page, because the map is not a list and a merchant who wants the
// rest wants the resource's own index, not more nodes.
const MerchantInstanceCap = 100

// MerchantInstance is one node that exists, named the way its owner named it.
type MerchantInstance struct {
	ID string `json:"id"`
	// Label is the id for a type KYC does not store, and the key for one it
	// does. A merchant writes document:d1 and reads back d1; the projection
	// writes role:<opaque id> and they would not recognise it.
	Label string `json:"label"`
	// Detail is what this instance carries, for the types where carrying
	// something is the whole difference between one instance and the next.
	//
	// Only roles have one today, and it is their own capabilities rather than
	// their effective set. The effective set already includes everything the
	// parents hold, so printing it on both ends of an extends edge would draw
	// the inheritance twice and make a child look like it declares what it only
	// inherits.
	Detail []string `json:"detail,omitempty"`
}

// MerchantInstanceEdge is one fact relating two drawn instances.
type MerchantInstanceEdge struct {
	FromType string `json:"from_type"`
	FromID   string `json:"from_id"`
	Label    string `json:"label"`
	ToType   string `json:"to_type"`
	ToID     string `json:"to_id"`
}

// MerchantInstanceType is every drawn instance of one type, and how many were
// not drawn.
type MerchantInstanceType struct {
	Type      string             `json:"type"`
	Instances []MerchantInstance `json:"instances"`
	// Total is the exact count, so the hint can say "100 of 3,204". Counting is
	// affordable because the same materialised set the cap ranks over is what
	// it counts, and both halves of that set are an index prefix.
	Total int64 `json:"total"`
	// Truncated says the drawing is a sample. Without it a capped type reads as
	// a complete one, which is the failure mode a cap introduces.
	Truncated bool `json:"truncated"`
}

// MerchantInstances is the instance layer of the map.
type MerchantInstances struct {
	Cap   int                    `json:"cap"`
	Types []MerchantInstanceType `json:"types"`
	// Edges relate one drawn instance to another. Without them the layer is a
	// fan of interchangeable chips, which is a worse answer than drawing
	// nothing: it asserts that three roles are the same.
	Edges []MerchantInstanceEdge `json:"edges"`
	// EdgesTruncated says max_edges was reached, so the relations are a sample.
	EdgesTruncated bool `json:"edges_truncated"`
}

// MaxMerchantInstanceEdges bounds the relations one map draws.
//
// The per-type cap does not bound these on its own: a hundred documents each
// sitting in one of a hundred projects is ten thousand containment edges, every
// one of them between two drawn nodes.
const MaxMerchantInstanceEdges = 600

// MerchantInstances returns what exists in a merchant's namespace, per type,
// capped.
//
// Read per request like the schema it accompanies. A cached instance layer
// would show a role that was deleted a minute ago sitting on a picture whose
// types are current, and a map that is half stale is worse than one that is
// slow.
func (s *Service) MerchantInstances(ctx context.Context, orgID string) (MerchantInstances, error) {
	rows, err := s.db.Q().ListMerchantInstances(ctx, sqlc.ListMerchantInstancesParams{
		Namespace: accessmodel.MerchantNamespace(orgID),
		OrgID:     orgID,
		PerType:   MerchantInstanceCap,
	})
	if err != nil {
		return MerchantInstances{}, err
	}

	// Customers are named only to a viewer who may already list them.
	//
	// The route is gated on app_access:read, which is the permission for the
	// model. app_user instances are not the model: they are the people in it,
	// and app_users:read is what governs those everywhere else. Dropping the one
	// type beats gating the whole layer, because a viewer who may see the roles
	// and the scopes should still see them.
	if _, err := s.RequireOrgPermission(ctx, orgID, "app_users:read"); err != nil {
		kept := rows[:0]
		for _, r := range rows {
			if r.NodeType != "app_user" {
				kept = append(kept, r)
			}
		}
		rows = kept
	}

	labels, detail, err := s.instanceLabels(ctx, orgID, rows)
	if err != nil {
		return MerchantInstances{}, err
	}

	out := MerchantInstances{Cap: MerchantInstanceCap, Types: []MerchantInstanceType{}}
	byType := map[string]*MerchantInstanceType{}
	for _, r := range rows {
		t, ok := byType[r.NodeType]
		if !ok {
			out.Types = append(out.Types, MerchantInstanceType{
				Type:      r.NodeType,
				Instances: []MerchantInstance{},
				Total:     r.Total,
				Truncated: r.Total > MerchantInstanceCap,
			})
			t = &out.Types[len(out.Types)-1]
			byType[r.NodeType] = t
		}
		key := r.NodeType + "\x00" + r.NodeID
		label := r.NodeID
		if got, ok := labels[key]; ok && got != "" {
			label = got
		}
		t.Instances = append(t.Instances, MerchantInstance{
			ID: r.NodeID, Label: label, Detail: detail[key],
		})
	}

	// The query orders by id, which for an opaque id is not the order the label
	// reads in. Sorting here rather than in SQL is what lets the label be
	// resolved from three different tables first.
	for i := range out.Types {
		sort.Slice(out.Types[i].Instances, func(a, b int) bool {
			return out.Types[i].Instances[a].Label < out.Types[i].Instances[b].Label
		})
	}

	edges, err := s.db.Q().ListMerchantInstanceEdges(ctx, sqlc.ListMerchantInstanceEdgesParams{
		Namespace: accessmodel.MerchantNamespace(orgID),
		OrgID:     orgID,
		PerType:   MerchantInstanceCap,
		MaxEdges:  MaxMerchantInstanceEdges + 1,
	})
	if err != nil {
		return MerchantInstances{}, err
	}
	// One over the bound, so reaching it is distinguishable from landing on it
	// exactly. Asking for the true count instead would mean counting a join the
	// cap exists to avoid materialising.
	if len(edges) > MaxMerchantInstanceEdges {
		edges = edges[:MaxMerchantInstanceEdges]
		out.EdgesTruncated = true
	}

	drawn := map[string]struct{}{}
	for _, t := range out.Types {
		for _, inst := range t.Instances {
			drawn[t.Type+"\x00"+inst.ID] = struct{}{}
		}
	}
	out.Edges = []MerchantInstanceEdge{}
	for _, e := range edges {
		// The SQL joins against the same cap, so this agrees with it already.
		// It is here because app_user instances are dropped after that query
		// runs, for a viewer without app_users:read, and an edge to a customer
		// who is no longer on the canvas would point at nothing.
		if _, ok := drawn[e.FromType+"\x00"+e.FromID]; !ok {
			continue
		}
		if _, ok := drawn[e.ToType+"\x00"+e.ToID]; !ok {
			continue
		}
		out.Edges = append(out.Edges, MerchantInstanceEdge(e))
	}
	return out, nil
}

// instanceLabels resolves the ids KYC stores into the names their author chose,
// and collects what an instance carries where that is what distinguishes it.
//
// Three tables and no more. A scope instance and a resource are the merchant's
// own ids, written by their product and meaningful to them already, so there is
// nothing to look up and nothing KYC could look it up in.
func (s *Service) instanceLabels(ctx context.Context, orgID string, rows []sqlc.ListMerchantInstancesRow) (map[string]string, map[string][]string, error) {
	out := map[string]string{}
	detail := map[string][]string{}

	var userIDs []string
	wantRoles, wantGroups := false, false
	for _, r := range rows {
		switch r.NodeType {
		case "app_user":
			userIDs = append(userIDs, r.NodeID)
		case "role":
			wantRoles = true
		case "group":
			wantGroups = true
		}
	}

	// Roles and groups are listed whole rather than by id. Both are authored by
	// hand and bounded by that, so the org-wide query is the cheaper one and it
	// already exists.
	if wantRoles {
		roles, err := s.db.Q().ListAppRoles(ctx, orgID)
		if err != nil {
			return nil, nil, err
		}
		for _, role := range roles {
			out["role\x00"+role.ID] = role.Key
			detail["role\x00"+role.ID] = role.OwnCapabilities
		}
	}
	if wantGroups {
		groups, err := s.db.Q().ListAppUserGroups(ctx, orgID)
		if err != nil {
			return nil, nil, err
		}
		for _, g := range groups {
			out["group\x00"+g.ID] = g.Key
		}
	}
	if len(userIDs) > 0 {
		users, err := s.db.Q().ListAppUserLabels(ctx, sqlc.ListAppUserLabelsParams{
			OrganisationID: orgID,
			Ids:            userIDs,
		})
		if err != nil {
			return nil, nil, err
		}
		for _, u := range users {
			out["app_user\x00"+u.ID] = appUserLabel(u)
		}
	}
	return out, detail, nil
}

// appUserLabel picks the name a merchant would recognise.
//
// external_id comes first because it is the id their own product uses, so it is
// the one that matches what they are looking at in their own admin. A display
// name is chosen by the customer and need not be unique; an email is neither
// always present nor always safe to put on a shared screen.
func appUserLabel(u sqlc.ListAppUserLabelsRow) string {
	if u.ExternalID.Valid && u.ExternalID.String != "" {
		return u.ExternalID.String
	}
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Email.Valid && u.Email.String != "" {
		return u.Email.String
	}
	return u.ID
}
