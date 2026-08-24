package accessmodel

import (
	"context"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// Namespace is KYC's own tenancy boundary. A merchant's model lives under
// org:<id>, which is what keeps their open vocabulary out of this one.
const Namespace = "kyc"

// Querier is the slice of the store the resolver needs. Narrow on purpose: the
// evaluator reads edges and nothing else, and a resolver that could reach the
// rest of the schema would eventually be made to.
type Querier interface {
	ListReachEdges(ctx context.Context, arg sqlc.ListReachEdgesParams) ([]sqlc.ReachEdge, error)
	ListReachEdgesForSubject(ctx context.Context, arg sqlc.ListReachEdgesForSubjectParams) ([]sqlc.ReachEdge, error)
}

// Resolver reads edges from Postgres.
//
// It answers exactly one question, an exact prefix of the edge table's primary
// key, so it stays an index lookup however large the table grows. Expiry is not
// filtered here: the evaluator is handed the time to decide against, so the
// query and the walk cannot disagree about what "now" means.
type Resolver struct {
	q Querier
}

// NewResolver returns a Resolver over the store.
func NewResolver(q Querier) *Resolver { return &Resolver{q: q} }

// Edges implements reach.Resolver.
func (r *Resolver) Edges(ctx context.Context, object reach.NodeRef, relation string) ([]reach.Edge, error) {
	rows, err := r.q.ListReachEdges(ctx, sqlc.ListReachEdgesParams{
		Namespace:  Namespace,
		ObjectType: object.Type,
		ObjectID:   object.ID,
		Relation:   relation,
	})
	if err != nil {
		return nil, err
	}
	out := make([]reach.Edge, 0, len(rows))
	for _, row := range rows {
		out = append(out, edgeFromRow(row))
	}
	return out, nil
}

// EdgesForSubject returns every edge naming one principal.
//
// This is the sweep: what offboarding runs when a person leaves, and what makes
// it safe for a key's owner edge to confer nothing. It is not on the decision
// path.
func (r *Resolver) EdgesForSubject(ctx context.Context, subject reach.NodeRef) ([]reach.Edge, error) {
	rows, err := r.q.ListReachEdgesForSubject(ctx, sqlc.ListReachEdgesForSubjectParams{
		Namespace:   Namespace,
		SubjectType: subject.Type,
		SubjectID:   subject.ID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]reach.Edge, 0, len(rows))
	for _, row := range rows {
		out = append(out, edgeFromRow(row))
	}
	return out, nil
}

func edgeFromRow(row sqlc.ReachEdge) reach.Edge {
	e := reach.Edge{
		Object:   reach.Node(row.ObjectType, row.ObjectID),
		Relation: row.Relation,
		Subject: reach.SubjectRef{
			Node:     reach.Node(row.SubjectType, row.SubjectID),
			Relation: row.SubjectRelation,
		},
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
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
	schema, err := Load()
	if err != nil {
		return nil, err
	}
	return reach.New(schema, NewResolver(q), opts...)
}
