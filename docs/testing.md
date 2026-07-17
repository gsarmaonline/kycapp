# Testing

Tests are required from Phase 1 onward — not bolted on after features ship.

## Principles

- Domain logic in `core/` is tested without HTTP where possible.
- HTTP handlers get table-driven API tests.
- Persistence tests use Postgres (testcontainers or a dedicated test DB), not mocks of SQL.
- Critical flows (signup, invite accept, authz check, entitlement effective set) have end-to-end API tests.

## Layers

| Layer | What | Tools |
| --- | --- | --- |
| Unit | Validation, effective entitlements, authz resolution | `go test`, table-driven |
| Store | Migrations + CRUD + uniqueness | Postgres + `go test` |
| API | `/v1` status codes, bodies, idempotency | `net/http/httptest` against `cmd/api` |
| Frontend (Phase 2+) | Ops console flows | Vitest + Testing Library |

## Must-cover scenarios (from Phase 1/2)

- Signup is atomic (failure rolls back; no orphan org/user)
- Signup idempotency with `Idempotency-Key`
- User can belong to multiple organisations
- Membership invite → accept → active
- Authz check: key and resource+action forms agree
- Authz denied when membership revoked or org suspended
- Effective entitlements: plan ∪ grant − deny
- Entitlement check vs permission check stay independent

## Frontend

The Vite + React ops console (`web/`) starts in **Phase 2**, alongside organisations/users/memberships APIs. Phase 0–1 are specs and backend skeleton only.
