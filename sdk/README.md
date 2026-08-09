# Merchant SDK

Client libraries for the **Integration API** — the 25 paths a merchant backend
calls with an organisation API key (`kyc_…`). See
[docs/api.md](../docs/api.md#merchant-integration-vs-operator-api) for the
Integration vs Operator split.

## Layers

| Layer | Status | What it is |
| --- | --- | --- |
| **Transport** | **Shipped** | Generated from the spec. Types + a typed HTTP client. |
| **Facade** | Not built | Hand-written ergonomics: organisation bound at construction, cached entitlement checks, an explicit failure policy. |

Only the transport layer exists today. It is generated, so it never drifts from
the API. Method names come from `operationId`, and every request body is a named
schema, so argument types can be imported and re-exported.

## Packages

| Path | Module / package | Generator |
| --- | --- | --- |
| `sdk/go` | `github.com/gsarmaonline/kyc/sdk/go` | `oapi-codegen` |
| `sdk/ts` | `@kyc/sdk` | `openapi-typescript` + `openapi-fetch` |

`sdk/go` is a **separate Go module** on purpose. Merchants importing it must not
inherit the API server's dependencies — pgx, River, Stripe, and Testcontainers.

## Regenerate

```bash
make sdk          # openapi subset -> Go + TypeScript clients
make test-sdk     # build/vet the Go module, typecheck the TS package
```

Generated output is **committed**. CI runs `make sdk-check`, which regenerates
and fails when the result differs from what is committed. So a spec change
cannot merge without its regenerated client.

Do not hand-edit `sdk/go/kyc/generated.go` or `sdk/ts/src/generated/`.

## Use it

Go:

```go
client, err := kyc.NewClient("https://kyc.example.com", kyc.WithAPIKey(key))
resp, err := client.CheckEntitlement(ctx, kyc.CheckEntitlementRequest{
    OrganisationId: orgID,
    Entitlement:    "premium_export",
})
```

TypeScript:

```ts
import createClient from "@kyc/sdk"

const client = createClient({ baseUrl, apiKey })
const { data, error } = await client.POST("/v1/entitlements/check", {
  body: { organisation_id: orgId, entitlement: "premium_export" },
})
```

## Spec rules the SDK depends on

`go test ./cmd/openapi-filter/` enforces both, so codegen stays usable:

1. Every Integration operation has a unique `operationId`. Without it the
   generator falls back to a path-derived name such as
   `PutV1OrganisationsIdAppUsersIngest`.
2. Every Integration request body is a `$ref` to `components/schemas`. Inline
   bodies generate anonymous types that callers cannot import.
