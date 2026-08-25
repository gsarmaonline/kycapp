package service

import (
	"context"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// KYC evaluating a merchant's authorisation.
//
// The merchant tier used to assemble a description of authority and hand it
// over for somebody else's backend to interpret. That was a workaround for KYC
// not holding the merchant's resources, and it put the merchant back in the
// business of building an authorisation layer, which is the thing they are here
// to avoid.
//
// So they write their ownership and containment edges, and ask. Everything the
// engine already does works immediately: wildcards, exclusions, expiry, group
// and role nesting, and the decision path.
//
// The tenancy boundary is the namespace and nothing else. Every edge query
// filters on it and a resolver carries exactly one, so a walk physically cannot
// read another merchant's edges. No name is reserved, because a name was never
// what kept them apart.

// MerchantEdge is one fact a merchant states about their own product.
type MerchantEdge struct {
	ObjectType string
	ObjectID   string
	Relation   string
	// SubjectType and SubjectID name the far end. SubjectRelation makes it a
	// userset: role:editor#holder is whoever holds that role.
	SubjectType     string
	SubjectID       string
	SubjectRelation string
	ExpiresAt       *time.Time
}

func (e MerchantEdge) node() reach.NodeRef {
	return reach.Node(e.ObjectType, e.ObjectID)
}

// merchantModel reads what a merchant declared, which is what their schema is
// derived from. It is read per request rather than cached, so a capability
// added a second ago is usable now; the two queries are small and indexed.
func (s *Service) merchantModel(ctx context.Context, orgID string) (accessmodel.MerchantModel, error) {
	kinds, err := s.db.Q().ListAppScopeTypes(ctx, orgID)
	if err != nil {
		return accessmodel.MerchantModel{}, err
	}
	caps, err := s.db.Q().ListAppCapabilities(ctx, orgID)
	if err != nil {
		return accessmodel.MerchantModel{}, err
	}
	m := accessmodel.MerchantModel{OrganisationID: orgID}
	for _, k := range kinds {
		m.ScopeKinds = append(m.ScopeKinds, k.Kind)
	}
	for _, c := range caps {
		m.CapabilityKeys = append(m.CapabilityKeys, c.Key)
	}
	return m, nil
}

// MerchantSchema returns the schema a merchant's own model resolves as.
//
// Derived from their rows rather than stored, so it cannot drift from the
// vocabulary they authored and there is no migration when they add a kind.
func (s *Service) MerchantSchema(ctx context.Context, orgID string) (*reach.Schema, error) {
	model, err := s.merchantModel(ctx, orgID)
	if err != nil {
		return nil, err
	}
	schema, err := accessmodel.MerchantSchema(model)
	if err != nil {
		return nil, apperr.Validation(err.Error())
	}
	return schema, nil
}

// WriteMerchantEdges records facts about a merchant's own resources.
//
// Writes are idempotent, so a merchant re-syncing after a failure converges
// rather than duplicating. That matters more here than anywhere else in the
// system: this is a write path their product runs on every resource create, and
// the first thing that goes wrong with one is a retry.
func (s *Service) WriteMerchantEdges(ctx context.Context, orgID string, edges []MerchantEdge) (int, error) {
	if len(edges) == 0 {
		return 0, apperr.Validation("no edges given")
	}
	if len(edges) > MaxMerchantEdgeBatch {
		return 0, apperr.Validation("too many edges in one write")
	}
	schema, err := s.MerchantSchema(ctx, orgID)
	if err != nil {
		return 0, err
	}
	ns := accessmodel.MerchantNamespace(orgID)

	// Validate the whole batch before writing any of it. A batch that fails
	// halfway leaves a graph nobody asked for, and a retry then has to reason
	// about which half landed.
	rows := make([]sqlc.WriteReachEdgeParams, 0, len(edges))
	for _, e := range edges {
		edge, err := validMerchantEdge(schema, e)
		if err != nil {
			return 0, err
		}
		p := accessmodel.RowFor(edge, "merchant")
		p.Namespace = ns
		rows = append(rows, p)
	}

	// One transaction, and the version bump inside it. A reader that sees the
	// new version has to be able to see every edge it counts, or a cache
	// refreshes against a graph that is still half written.
	if err := s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		for _, p := range rows {
			if err := q.WriteReachEdge(ctx, p); err != nil {
				return err
			}
		}
		_, err := q.BumpReachNamespaceVersion(ctx, ns)
		return err
	}); err != nil {
		return 0, err
	}
	return len(edges), nil
}

// DeleteMerchantEdge removes one fact. Deleting a resource in a merchant's
// product has to remove its edges, or access outlives the thing it was about.
func (s *Service) DeleteMerchantEdge(ctx context.Context, orgID string, e MerchantEdge) error {
	schema, err := s.MerchantSchema(ctx, orgID)
	if err != nil {
		return err
	}
	if _, err := validMerchantEdge(schema, e); err != nil {
		return err
	}
	ns := accessmodel.MerchantNamespace(orgID)
	// A delete is the case the version exists for. It moves no timestamp on any
	// surviving row, so without the bump a revocation is invisible to every
	// cache and the stale answer is the *wider* one.
	return s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.DeleteReachEdge(ctx, sqlc.DeleteReachEdgeParams{
			Namespace:       ns,
			ObjectType:      e.ObjectType,
			ObjectID:        e.ObjectID,
			Relation:        e.Relation,
			SubjectType:     e.SubjectType,
			SubjectID:       e.SubjectID,
			SubjectRelation: e.SubjectRelation,
		}); err != nil {
			return err
		}
		_, err := q.BumpReachNamespaceVersion(ctx, ns)
		return err
	})
}

// MerchantGraphVersion is what a merchant caches against.
//
// It answers "has anything changed since the number I hold", which a timestamp
// on the edge table cannot: a delete moves no timestamp, and a revocation is
// exactly the change a cache must not miss.
func (s *Service) MerchantGraphVersion(ctx context.Context, orgID string) (int64, error) {
	return s.db.Q().GetReachNamespaceVersion(ctx, accessmodel.MerchantNamespace(orgID))
}

// MaxMerchantEdgeBatch bounds one write. Generous for a sync loop and small
// enough that one request cannot hold a connection indefinitely.
const MaxMerchantEdgeBatch = 500

// validMerchantEdge checks an edge against the merchant's own schema.
//
// The check is not a security boundary: an edge naming an undeclared relation
// reaches nothing, because no rule reads it. It is there so the failure is
// loud at write time rather than silent at check time, which is the same reason
// capabilities are declared at all.
func validMerchantEdge(schema *reach.Schema, e MerchantEdge) (reach.Edge, error) {
	edge := reach.Edge{
		Object:   e.node(),
		Relation: strings.TrimSpace(e.Relation),
		Subject: reach.SubjectRef{
			Node:     reach.Node(e.SubjectType, e.SubjectID),
			Relation: strings.TrimSpace(e.SubjectRelation),
		},
		ExpiresAt: e.ExpiresAt,
	}
	if err := edge.Validate(); err != nil {
		return reach.Edge{}, apperr.Validation(err.Error())
	}
	if _, ok := schema.Type(e.ObjectType); !ok {
		return reach.Edge{}, apperr.Validation("unknown type: " + e.ObjectType)
	}
	if _, ok := schema.Type(e.SubjectType); !ok {
		return reach.Edge{}, apperr.Validation("unknown type: " + e.SubjectType)
	}
	if _, ok := schema.Relation(edge.Relation); !ok {
		return reach.Edge{}, apperr.Validation("unknown relation: " + edge.Relation)
	}
	return edge, nil
}

// MerchantObjects is what a customer can reach, for a listing page.
type MerchantObjects struct {
	ObjectIDs []string `json:"object_ids"`
	// All means a wildcard grant covers every object of this type, including
	// ones no edge names. ObjectIDs is then a lower bound, and a backend that
	// filtered a page by it would wrongly hide rows.
	All bool `json:"all"`
	// Truncated means the candidate walk hit its bound, so the answer is a
	// subset. Saying so beats a short list that reads as complete.
	Truncated bool `json:"truncated"`
}

// ListMerchantObjects answers "what can this customer see?".
//
// The alternative is a check per row, which is what makes an authorisation
// service unusable on a listing page: fifty documents become fifty walks, and
// ten thousand cannot be rendered at all.
func (s *Service) ListMerchantObjects(ctx context.Context, orgID, subjectType, subjectID, action, resourceType string) (MerchantObjects, error) {
	e, err := s.merchantEvaluator(ctx, orgID, action)
	if err != nil {
		return MerchantObjects{}, err
	}
	got, err := e.ListObjects(ctx, reach.Node(subjectType, subjectID), action, resourceType, decideNow())
	if err != nil {
		return MerchantObjects{}, apperr.Validation(err.Error())
	}
	out := MerchantObjects{All: got.All, Truncated: got.Truncated, ObjectIDs: []string{}}
	for _, n := range got.Objects {
		out.ObjectIDs = append(out.ObjectIDs, n.ID)
	}
	return out, nil
}

// MerchantSubjects is who reaches one object, for a share dialog or an audit.
type MerchantSubjects struct {
	Subjects []MerchantSubject `json:"subjects"`
	// All means an everyone grant reaches this, so Subjects is a lower bound.
	All       bool `json:"all"`
	Truncated bool `json:"truncated"`
}

// MerchantSubject is one principal that reaches an object.
type MerchantSubject struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// ListMerchantSubjects answers "who can see this?".
func (s *Service) ListMerchantSubjects(ctx context.Context, orgID, action, resourceType, resourceID string) (MerchantSubjects, error) {
	e, err := s.merchantEvaluator(ctx, orgID, action)
	if err != nil {
		return MerchantSubjects{}, err
	}
	got, err := e.ListSubjects(ctx, reach.Node(resourceType, resourceID), action, decideNow())
	if err != nil {
		return MerchantSubjects{}, apperr.Validation(err.Error())
	}
	out := MerchantSubjects{All: got.All, Truncated: got.Truncated, Subjects: []MerchantSubject{}}
	for _, n := range got.Subjects {
		out.Subjects = append(out.Subjects, MerchantSubject{Type: n.Type, ID: n.ID})
	}
	return out, nil
}

// merchantEvaluator builds the evaluator for one merchant's namespace, after
// checking the action is one they declared. An undeclared action reaches
// nothing, and saying so beats an empty list the caller has to interpret.
func (s *Service) merchantEvaluator(ctx context.Context, orgID, action string) (*reach.Evaluator, error) {
	schema, err := s.MerchantSchema(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if !schema.HasAction(action) {
		return nil, apperr.Validation("unknown action: " + action)
	}
	return reach.New(schema, accessmodel.NewResolverIn(s.db.Q(), accessmodel.MerchantNamespace(orgID)))
}

// MerchantDecision is one answer, with the route the walk took to it.
type MerchantDecision struct {
	Allowed bool      `json:"allowed"`
	Reason  string    `json:"reason"`
	Path    []PathHop `json:"path"`
}

// CheckMerchant answers one question against a merchant's own graph.
//
// This is the call that replaces an authorisation layer. The path comes back
// unredacted, unlike the operator-facing one: a merchant asking about their own
// namespace already owns every node in the answer, so there is nothing to
// withhold and the route is the most useful part.
func (s *Service) CheckMerchant(ctx context.Context, orgID, subjectType, subjectID, action, resourceType, resourceID string) (MerchantDecision, error) {
	schema, err := s.MerchantSchema(ctx, orgID)
	if err != nil {
		return MerchantDecision{}, err
	}
	if !schema.HasAction(action) {
		// A closed vocabulary per namespace: an action nobody declared reaches
		// nothing, and saying so beats a bare denial the caller has to guess at.
		return MerchantDecision{}, apperr.Validation("unknown action: " + action)
	}
	e, err := reach.New(schema, accessmodel.NewResolverIn(s.db.Q(), accessmodel.MerchantNamespace(orgID)))
	if err != nil {
		return MerchantDecision{}, err
	}
	d, err := e.Check(ctx, reach.Request{
		Subject:  reach.Node(subjectType, subjectID),
		Action:   action,
		Resource: reach.Node(resourceType, resourceID),
	}, decideNow())
	if err != nil {
		return MerchantDecision{}, apperr.Validation(err.Error())
	}
	return MerchantDecision{
		Allowed: d.Allowed,
		Reason:  string(d.Reason),
		Path:    filterPath(d.Path, nil, true),
	}, nil
}
