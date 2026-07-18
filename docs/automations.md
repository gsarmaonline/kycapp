# Automations (merchant workflows)

**Decision:** User-authored automations live **inside KYC**. Merchants and the KYC team author the same org-scoped rules. Conditions stay **simple**. Execution uses **River** (Go + existing Postgres) — not Temporal.

## Why this shape

| Need | Choice |
| --- | --- |
| Authors = merchants + team | In-app CRUD UI (forms, not a separate n8n) |
| Simple conditions | JSON rule DSL evaluated in Go |
| Series of actions | Ordered action list on the rule |
| Ops / scale (for now) | River job queue on KYC Postgres; one worker process |

Temporal / Restate / Hatchet are deferred until we need long-running waits, human-in-the-loop, or complex branching.

## V1 model

Org-scoped **Automation** (name optional):

```json
{
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

### Triggers (v1)

| Trigger | When |
| --- | --- |
| `app_user.created` | After app user create |
| `app_user.updated` | After app user update (optional; can wait) |

Add more later (`membership.invited`, `subscription.updated`, …) without changing the DSL shape.

### Conditions

- Combinator: `all` (AND) only in v1; `any` (OR) later if needed.
- Ops: `eq`, `neq`, `exists`, `not_exists` (enough for attributes / status).
- Fields: dotted paths on a small event payload (`status`, `attributes.<key>`, …).

### Actions (v1)

| Type | Behavior |
| --- | --- |
| `send_email` | Render org email template + branding (delivery provider can be stub/log until SMTP/ESP exists) |
| `set_attribute` | Optional later |

Unknown action types fail the run with a clear error (no silent skip).

## Runtime

1. Domain event in API/service → enqueue River job `{ org_id, trigger, payload }`.
2. Worker loads enabled automations for that org + trigger.
3. For each rule: evaluate conditions → run actions in order.
4. Persist run log (success / skip / error) for merchant visibility.

Same worker binary serves platform and merchant automations; tenancy is always `organisation_id`.

## UI (inside KYC)

Under `/orgs/:orgId/automations`:

- List / create / edit / enable-disable / delete
- Form: trigger select → condition rows → action rows (template picker for email)
- Optional “Runs” list later

No drag-and-drop canvas in v1.

## Local & Railway

| Env | Pieces |
| --- | --- |
| Local | Existing Postgres + `api` + `worker` (River consumers) |
| Railway | Same DB as API; add **worker** service when automations ship — no Temporal cluster |

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
