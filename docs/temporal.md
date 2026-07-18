# Temporal (workflows)

Durable workflow execution for background jobs (checks, email, billing retries, etc.).

| Piece | Role |
| --- | --- |
| **Temporal server** | Persists workflow history; schedules tasks |
| **Worker** (`cmd/worker`) | Runs workflow/activity code (`internal/workflows`) |
| **Temporal UI** | Observe runs (not a visual authoring builder) |

Task queue: `kyc` (override with `TEMPORAL_TASK_QUEUE`).

## Local (Docker Compose)

`docker compose up --build -d` starts Temporal alongside the app:

| URL / port | Service |
| --- | --- |
| http://localhost:8080 | App + API |
| http://localhost:8088 | Temporal Web UI |
| `localhost:7233` | Temporal gRPC (workers / CLI) |

Temporal uses its **own** Postgres (`temporal-postgres`). Do not point it at the KYC `DATABASE_URL`.

Smoke-test the stub workflow (needs [Temporal CLI](https://docs.temporal.io/cli)):

```bash
temporal workflow start \
  --address localhost:7233 \
  --task-queue kyc \
  --type PingWorkflow \
  --workflow-id ping-1 \
  --input '"kyc"'

temporal workflow show --address localhost:7233 --workflow-id ping-1
```

Or run the worker on the host against compose Temporal:

```bash
export TEMPORAL_ADDRESS=localhost:7233
go run ./cmd/worker
```

## Railway

Two options:

### A. Temporal Cloud (recommended for production)

1. Create a namespace in [Temporal Cloud](https://cloud.temporal.io).
2. Add a **worker** service from this repo; Config-as-code → `/railway.worker.toml`.
3. Set on the worker:

| Variable | Value |
| --- | --- |
| `TEMPORAL_ADDRESS` | Cloud gRPC endpoint (from Temporal Cloud) |
| `TEMPORAL_NAMESPACE` | Cloud namespace |
| `TEMPORAL_TASK_QUEUE` | `kyc` |

Use mTLS / API key as Temporal Cloud requires (wire into `cmd/worker` when you enable Cloud).

Keep the API private; the worker needs only outbound access to Temporal Cloud (and your DB/APIs as activities need them).

### B. Self-host Temporal on Railway

1. Deploy Railway’s [Temporal template](https://railway.com/deploy/temporalio) **into the same project** (server + its Postgres ± UI).
2. Add this repo’s **worker** service (`/railway.worker.toml`).
3. Set:

| Variable | Value |
| --- | --- |
| `TEMPORAL_ADDRESS` | `${{Temporal Auto Setup.RAILWAY_PRIVATE_DOMAIN}}:7233` (name may vary — use your Temporal service’s private hostname) |
| `TEMPORAL_NAMESPACE` | `default` |
| `TEMPORAL_TASK_QUEUE` | `kyc` |

Do **not** expose Temporal gRPC publicly. Optional: public Temporal UI only if you need ops access (prefer private + VPN / Railway TCP proxy).

| Service | Config file |
| --- | --- |
| api | [`railway.api.toml`](../railway.api.toml) |
| web | [`railway.web.toml`](../railway.web.toml) |
| worker | [`railway.worker.toml`](../railway.worker.toml) |

## Visual workflow builders

Temporal is **code-first**. The official Temporal UI is for **monitoring / replay**, not designing graphs.

| Option | Fit |
| --- | --- |
| **Write workflows in Go** (`internal/workflows`) | Default for KYC — type-safe, testable, matches Temporal’s model |
| **[Workflow Builder](https://www.workflowbuilder.io/)** ([synergycodes/workflowbuilder](https://github.com/synergycodes/workflowbuilder)) | Embeddable React canvas; exports JSON executed by a Temporal interpreter worker. Best if product users need drag-and-drop automations |
| **Camunda / n8n / similar** | Strong visual builders, but **not** Temporal — different runtime |

There is no first-party Temporal “BPMN designer.” For eng-owned jobs (billing, KYC pipelines), keep code workflows. Add Workflow Builder later only if merchants need to author flows in-product.

## Adding a real workflow

1. Define workflow + activities in `internal/workflows`.
2. Register them in `Register`.
3. Start executions from the API (Temporal client) or CLI.
4. Redeploy the **worker** (API restart alone does not pick up new activity code).
