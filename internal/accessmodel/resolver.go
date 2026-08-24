package accessmodel

import (
	"context"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// Namespace is KYC's own tenancy boundary. A merchant's model lives under
// MerchantNamespace(orgID), which is what keeps their open vocabulary out of
// this one.
const Namespace = "kyc"

// MerchantNamespace is where one merchant's own model lives.
//
// The isolation between namespaces is structural, not a rule anyone enforces.
// Every edge query filters on namespace and a resolver carries exactly one, so
// a walk physically cannot read another namespace's edges. Nothing needs to
// forbid a merchant from naming a scope kind "global": inside org:acme it would
// be global:x, reaching nothing outside, because no edge crosses.
//
// That is worth stating because the alternative keeps suggesting itself. A
// blacklist of reserved names implies the name carries power, and a system that
// believes that grows a policy language bolted to the side of the graph.
// TestNamespacesCannotSeeEachOther is what actually holds this.
func MerchantNamespace(orgID string) string { return "org:" + orgID }

// Querier is the slice of the store the resolver needs. Narrow on purpose: the
// evaluator reads edges and nothing else, and a resolver that could reach the
// rest of the schema would eventually be made to.
type Querier interface {
	ListLiveEdges(ctx context.Context, arg sqlc.ListLiveEdgesParams) ([]sqlc.ReachEdgesLive, error)
	ListLiveEdgesForSubject(ctx context.Context, arg sqlc.ListLiveEdgesForSubjectParams) ([]sqlc.ReachEdgesLive, error)
	ListReachEdges(ctx context.Context, arg sqlc.ListReachEdgesParams) ([]sqlc.ReachEdge, error)
	ListReachEdgesForSubject(ctx context.Context, arg sqlc.ListReachEdgesForSubjectParams) ([]sqlc.ReachEdge, error)
}

// Source says which rows a resolver reads.
type Source int

const (
	// SourceLive reads reach_edges_live: the current authorisation tables
	// presented as edges, unioned with anything written directly. This is what
	// production uses during the cutover, and it is what makes the cutover safe
	// -- no write path has to be changed, so none can be missed.
	SourceLive Source = iota
	// SourceEdges reads reach_edges alone. Used by the projection's own tests,
	// which would otherwise pass whether or not the projection did anything.
	SourceEdges
)

// Resolver reads edges from Postgres.
//
// It answers exactly one question, an exact prefix of the edge table's primary
// key, so it stays an index lookup however large the table grows. Expiry is not
// filtered here: the evaluator is handed the time to decide against, so the
// query and the walk cannot disagree about what "now" means.
type Resolver struct {
	q      Querier
	source Source
	// namespace is the only thing separating one tenant's graph from another's.
	// It is a field rather than a constant because a merchant's model is the
	// same engine over the same table, and the constant was the single line
	// pinning all of this to one tenant.
	namespace string
}

// NewResolver returns a Resolver reading the live view of KYC's own namespace.
func NewResolver(q Querier) *Resolver {
	return &Resolver{q: q, source: SourceLive, namespace: Namespace}
}

// NewResolverIn returns a Resolver over one namespace, reading written edges
// only. A merchant's model has no legacy tables behind a view, so there is
// nothing for SourceLive to add.
func NewResolverIn(q Querier, namespace string) *Resolver {
	return &Resolver{q: q, source: SourceEdges, namespace: namespace}
}

// NewResolverFrom returns a Resolver reading the named source.
func NewResolverFrom(q Querier, source Source) *Resolver {
	return &Resolver{q: q, source: source, namespace: Namespace}
}

// Edges implements reach.Resolver.
func (r *Resolver) Edges(ctx context.Context, object reach.NodeRef, relation string) ([]reach.Edge, error) {
	if r.source == SourceEdges {
		rows, err := r.q.ListReachEdges(ctx, sqlc.ListReachEdgesParams{
			Namespace:  r.namespace,
			ObjectType: object.Type,
			ObjectID:   object.ID,
			Relation:   relation,
		})
		if err != nil {
			return nil, err
		}
		out := make([]reach.Edge, 0, len(rows))
		for _, row := range rows {
			out = append(out, edgeFromRow(row.ObjectType, row.ObjectID, row.Relation,
				row.SubjectType, row.SubjectID, row.SubjectRelation, row.ExpiresAt))
		}
		return out, nil
	}

	rows, err := r.q.ListLiveEdges(ctx, sqlc.ListLiveEdgesParams{
		Namespace:  r.namespace,
		ObjectType: object.Type,
		ObjectID:   object.ID,
		Relation:   relation,
	})
	if err != nil {
		return nil, err
	}
	out := make([]reach.Edge, 0, len(rows))
	for _, row := range rows {
		out = append(out, edgeFromRow(row.ObjectType, row.ObjectID, row.Relation,
			row.SubjectType, row.SubjectID, row.SubjectRelation, row.ExpiresAt))
	}
	return out, nil
}

// EdgesForSubject returns every edge naming one principal.
//
// This is the sweep: what offboarding runs when a person leaves, and what makes
// it safe for a key's owner edge to confer nothing. It is not on the decision
// path.
func (r *Resolver) EdgesForSubject(ctx context.Context, subject reach.NodeRef) ([]reach.Edge, error) {
	if r.source == SourceEdges {
		rows, err := r.q.ListReachEdgesForSubject(ctx, sqlc.ListReachEdgesForSubjectParams{
			Namespace:   r.namespace,
			SubjectType: subject.Type,
			SubjectID:   subject.ID,
		})
		if err != nil {
			return nil, err
		}
		out := make([]reach.Edge, 0, len(rows))
		for _, row := range rows {
			out = append(out, edgeFromRow(row.ObjectType, row.ObjectID, row.Relation,
				row.SubjectType, row.SubjectID, row.SubjectRelation, row.ExpiresAt))
		}
		return out, nil
	}
	rows, err := r.q.ListLiveEdgesForSubject(ctx, sqlc.ListLiveEdgesForSubjectParams{
		Namespace:   r.namespace,
		SubjectType: subject.Type,
		SubjectID:   subject.ID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]reach.Edge, 0, len(rows))
	for _, row := range rows {
		out = append(out, edgeFromRow(row.ObjectType, row.ObjectID, row.Relation,
			row.SubjectType, row.SubjectID, row.SubjectRelation, row.ExpiresAt))
	}
	return out, nil
}

func edgeFromRow(objectType, objectID, relation, subjectType, subjectID, subjectRelation string, expires pgtype.Timestamptz) reach.Edge {
	e := reach.Edge{
		Object:   reach.Node(objectType, objectID),
		Relation: relation,
		Subject: reach.SubjectRef{
			Node:     reach.Node(subjectType, subjectID),
			Relation: subjectRelation,
		},
	}
	if expires.Valid {
		t := expires.Time
		e.ExpiresAt = &t
	}
	return e
}

// RowFor turns an edge into the parameters that write it, so a caller never
// assembles those columns by hand and cannot put the subject in the object's
// place.
func RowFor(e reach.Edge, source string) sqlc.WriteReachEdgeParams {
	p := sqlc.WriteReachEdgeParams{
		Namespace:       Namespace,
		ObjectType:      e.Object.Type,
		ObjectID:        e.Object.ID,
		Relation:        e.Relation,
		SubjectType:     e.Subject.Node.Type,
		SubjectID:       e.Subject.Node.ID,
		SubjectRelation: e.Subject.Relation,
		Source:          source,
	}
	if e.ExpiresAt != nil {
		p.ExpiresAt.Time = *e.ExpiresAt
		p.ExpiresAt.Valid = true
	}
	return p
}

// NewEvaluator builds an evaluator over the store, with KYC's schema.
func NewEvaluator(q Querier, opts ...reach.Option) (*reach.Evaluator, error) {
	return NewEvaluatorFrom(q, SourceLive, opts...)
}

// NewEvaluatorFrom builds an evaluator reading the named source.
func NewEvaluatorFrom(q Querier, source Source, opts ...reach.Option) (*reach.Evaluator, error) {
	schema, err := Load()
	if err != nil {
		return nil, err
	}
	return reach.New(schema, NewResolverFrom(q, source), opts...)
}
