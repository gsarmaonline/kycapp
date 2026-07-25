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
| Local e2e | Seed core models + automation happy path | `make test-e2e` (testcontainers; see below) |
| Frontend (Phase 2+) | Ops console flows | Vitest + Testing Library |

## Local e2e (`make test-e2e`)

Runs **only on your machine / CI with Docker** — never against Railway production.

| Piece | Stub / local stand-in |
| --- | --- |
| Postgres | testcontainers `postgres:16-alpine` |
| Stripe / payments | default `noop` provider (no network) |
| Resend / email | `mailer.Recording` (in-memory) |
| River worker | `syncEnqueuer` processes automation jobs inline |

Entry point: `TestE2ELocalHappyPath` in `internal/service/e2e_local_test.go`.

It creates an org and populates branding, email templates, attributes, app users, product features/plans, outbound + inbound webhooks, an automation (`send_email` on `app_user.created`), an org API key, and a membership invite — then asserts the automation run, recorded email, and shared `{{path}}` webhook template rendering.

Outbound `call_webhook` delivery to loopback is intentionally not exercised (SSRF host guard); webhook **templates** are still validated via `automations.BuildWebhookBody`.

```bash
make test-e2e
# or: go test ./internal/service/ -run 'TestE2ELocal' -count=1 -timeout 3m -v
```

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
