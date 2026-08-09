// Package access is the portable authorisation evaluator for KYC.
//
// It is a separate Go module with **zero dependencies** on purpose. The same
// logic has to run in three places: the KYC API, the Go SDK inside a merchant's
// backend, and (ported) the TypeScript SDK. A merchant caches a principal's
// grant set and evaluates locally, so this package must never reach a database,
// a clock, or KYC's own types.
//
// Everything here is a pure function. Time is passed in.
//
// The model, and where it is going, is documented in docs/access-control.md.
//
//	Capability   a namespaced verb            kyc / members:invite
//	Scope        a set of resources           global | organisation:acme
//	Role         a named capability set       expanded at write time
//	Grant        binds capabilities to a      (scope, capabilities, expiry)
//	             principal within a scope
//
// Two rules shape the whole design:
//
//   - Grants are additive. There are no deny rules, so a union of grants is
//     commutative and role inheritance needs no precedence rules.
//   - Denials distinguish out-of-scope (which callers map to 404, so tenants
//     cannot be enumerated) from missing-capability (403).
package access
