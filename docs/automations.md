# Automations (merchant workflows)

**Decision:** User-authored automations live **inside KYC**. Merchants and the KYC team author the same org-scoped rules. Conditions stay **simple**. Execution uses **River** (Go + existing Postgres) — not Temporal.

## Why this shape

| Need | Choice |
| --- | --- |
| Authors = merchants + team | In-app visual DAG editor (React Flow), not a separate n8n |
| Simple conditions | JSON rule DSL evaluated in Go |
| Series of actions | Ordered action list on the rule |
| Ops / scale (for now) | River job queue on KYC Postgres; one worker process |

Temporal / Restate / Hatchet are deferred until we need long-running waits, human-in-the-loop, or complex branching.

## V1 model

Org-scoped **Automation** (**name required**):

```json
{
  "name": "Welcome AU users",
  "trigger": "app_user.created",
  "enabled": true,
  "conditions": {
    "all": [
      { "field": "attributes.country", "op": "eq", "value": "AU" }
    ]
  },
  "actions": [
    { "type": "send_email", "template_key": "welcome" }
  ]
}
```

### Catalog (single source of truth)

Triggers, actions, ops, and condition fields are defined in `core/automations` (registry) and exposed per org as:

`GET /v1/organisations/{id}/automations/catalog`

| Key | Source |
| --- | --- |
| `triggers` | Registered trigger descriptors |
| `actions` | Registered action types + params |
| `ops` | Condition operators |
| `condition_fields` | Base app-user payload fields + **all active org attribute definitions** as `attributes.<key>` |

The merchant UI loads this catalog for dropdowns. Create/update validates condition fields against the same set.

### Triggers (v1)

| Trigger | When |
| --- | --- |
| `app_user.created` | After app user create (including ingest insert) |
| `app_user.updated` | After app user update (including ingest upsert) |

Ingest (`PUT …/app-users/ingest`) merges attributes and enqueues the same triggers, so conditions on `attributes.*` work for externally authored profiles. Add more later (`membership.invited`, `subscription.updated`, …) by registering them in `core/automations`.

### Conditions

- Combinator: `all` (AND) only in v1; `any` (OR) later if needed.
- Ops: `eq`, `neq`, `exists`, `not_exists` (enough for attributes / status).
- Fields: base user fields (`id`, `email`, `display_name`, `status`, `external_id`) plus every active attribute definition.

### Actions (v1)

| Type | Behavior |
| --- | --- |
| `send_email` | Render org email template + branding, deliver via Mailer (`EMAIL_PROVIDER=resend` or `noop`) |
| `set_attribute` | Optional later |

Unknown action types fail the run with a clear error (no silent skip). Register new actions in `core/automations` and implement execute in the service worker path.

**Topology is fixed for v1:** one trigger → AND conditions → **all** actions in the list (in order). There is no “this condition → only these actions” branching. Add multiple actions with **Add action**; they all run when the conditions match. Per-path branches would need a richer DSL later.

## Runtime

1. Domain event in API/service → enqueue River job `{ org_id, trigger, payload }`.
2. Worker loads enabled automations for that org + trigger.
3. For each rule: evaluate conditions → run actions in order.
4. Persist run log (success / skip / error) for merchant visibility.

Same worker binary serves platform and merchant automations; tenancy is always `organisation_id`.

## UI (inside KYC)

Under `/orgs/:orgId/automations`:

- List / create / edit / enable-disable / delete
- **Visual DAG** (React Flow): trigger → conditions (AND fan-out) → ordered actions
- Node fields edit inline; add condition/action from the toolbar
- Show page renders the same DAG read-only, plus recent runs

## Local & Railway

| Env | Pieces |
| --- | --- |
| Local | Existing Postgres + `api` + `worker` (River consumers); email defaults to `noop` |
| Railway | Same DB as API + **worker**; set `EMAIL_PROVIDER=resend`, `RESEND_API_KEY`, `EMAIL_FROM` on **worker** (and api if it sends later) |

### Email delivery

```text
EMAIL_PROVIDER=noop|resend   # default noop (log only)
RESEND_API_KEY=re_...
EMAIL_FROM=KYC <mail@yourdomain.com>   # verified domain in Resend
```

Automations run in the **worker**, so Resend env must be on that service.

## Out of scope (v1)

- Visual graph builder
- Multi-branch trees, delays/schedules, human approval
- Cross-org / marketplace automations
- Calling arbitrary HTTP webhooks (add when we have allowlists + secrets)

## Implementation order

1. Spec (this doc) — **done**
2. Remove Temporal scaffolding — **done**
3. Migration + sqlc for `automations` (+ run log) — **done** (`000014`)
4. `core/automations` validate + evaluate — **done**
5. River worker + enqueue from `app_user` create/update — **done** (`cmd/worker`, compose `worker`)
6. REST + merchant UI — **done** (`/orgs/:orgId/automations`)
