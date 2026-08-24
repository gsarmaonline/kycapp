// Package reach is a relation-graph authorisation engine.
//
// Every access question is a path question: may this subject perform this
// action on this resource? Subjects, groups, roles, containers, tags and
// actions are all nodes. Grouping, containment, ownership and implication are
// all edges. One walk answers everything.
//
// The model has two primitives and one flag:
//
//	Object      a node: a type and an id            document:d1
//	Edge        one fact joining two nodes          document:d1 #parent folder:f2
//	transitive  a flag on a relation; the walk      relation member : transitive
//	            follows it to closure
//
// Read an edge as "A's relation is B". Everything else in this package
// declares what the edges mean and how far to follow them.
//
// # Why there is no attribute mechanism
//
// A tag, a department and a classification are each a node, and to carry one is
// an edge. A rule then connects the two sides. This costs the application a
// dual write, and buys a model with exactly one kind of fact: no predicate
// index, no second path through the evaluator, and no separate subset check for
// delegation.
//
// # Why the grammar is closed
//
// Anyone declares a new type, action or relation. Nobody adds an operator. The
// grammar is exactly union, traversal, subtraction, and a reference to another
// rule on the same type. There is no comparison operator and no function call,
// so the delegation subset question never becomes a satisfiability problem.
// Graph walks cost performance, which is an engineering problem.
// Undecidability would not have been.
//
// # Portability
//
// This package has no dependencies outside the standard library, reaches no
// database and reads no clock. Time is passed in. Edges the evaluator does not
// hold arrive through a [Resolver], so the same walk runs in the platform and
// inside a tenant's own backend against their own resource graph.
package reach
