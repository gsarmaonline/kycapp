# Data model

KYC is the system of record for **organisations**, their **users**, **authorisation**, and **billing entitlements**.

## Entity relationship

```mermaid
erDiagram
  Organisation ||--o{ Membership : has
  User ||--o{ Membership : has
  Role ||--o{ Membership : assigned
  Organisation ||--o{ Role : scoped
  Role ||--o{ RolePermission : grants
  Permission ||--o{ RolePermission : used_by
  Plan ||--o{ PlanEntitlement : includes
  Entitlement ||--o{ PlanEntitlement : used_by
  Organisation ||--o| Subscription : has
  Plan ||--o{ Subscription : of
  Organisation ||--o{ OrganisationEntitlement : overrides
  Entitlement ||--o{ OrganisationEntitlement : used_by
```

## Naming

| Term people say | Stored as | Notes |
| --- | --- | --- |
| Merchant / tenant / business / workspace | **Organisation** | Hub entity |
| Person / login identity | **User** | Global; many orgs via membership |
| Seat | **Membership** | User ↔ organisation + role |
| Billing account | Organisation + **Subscription** | No separate Account in v1 |

A future CRM “customer” (leads, pipeline) is a later layer — not a rename of Organisation.

---

## Objects

### Organisation

Tenant hub. Everything else hangs off this record.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string (ULID/UUID) | PK |
| `name` | string | Display name |
| `slug` | string | Unique URL-safe identifier |
| `status` | enum | `active` \| `suspended` \| `archived` |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

### User

Global person. Email is unique across the system.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | PK |
| `email` | string | Unique |
| `name` | string | |
| `status` | enum | `active` \| `disabled` |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

### Membership

Links a user to an organisation with a role.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | PK |
| `organisation_id` | string | FK → Organisation |
| `user_id` | string | FK → User |
| `role_id` | string | FK → Role |
| `status` | enum | `invited` \| `active` \| `revoked` |
| `created_at` | timestamptz | |

Unique `(organisation_id, user_id)`.

### Role

Organisation-scoped. System roles are seeded at signup.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | PK |
| `organisation_id` | string | FK → Organisation |
| `key` | string | e.g. `owner`, `admin`, `member` |
| `name` | string | Display name |
| `description` | string | Optional |
| `is_system` | bool | Seeded roles are not deletable |

Unique `(organisation_id, key)`. Default system roles: `owner`, `admin`, `member`.

### Permission

Global catalog of what a **user** may do inside an organisation (RBAC). Not the same as Entitlement.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | PK |
| `key` | string | Unique; conventionally `{resource}:{action}` |
| `resource` | string | e.g. `members`, `billing` |
| `action` | string | e.g. `invite`, `manage` |
| `category` | string | Admin UI grouping: `Access`, `Billing`, `Admin` |
| `description` | string | Human explanation |
| `is_system` | bool | Seeded; key/resource/action immutable |

Constraints: unique `key`; unique `(resource, action)`.

#### Seed catalog (v1)

| key | resource | action | category |
| --- | --- | --- | --- |
| `organisation:read` | organisation | read | Admin |
| `organisation:update` | organisation | update | Admin |
| `members:read` | members | read | Access |
| `members:invite` | members | invite | Access |
| `members:remove` | members | remove | Access |
| `roles:read` | roles | read | Access |
| `roles:manage` | roles | manage | Access |
| `billing:read` | billing | read | Billing |
| `billing:manage` | billing | manage | Billing |

### RolePermission

Join: Role ↔ Permission (many-to-many).

| Field | Type |
| --- | --- |
| `role_id` | FK → Role |
| `permission_id` | FK → Permission |

### Plan

Billing catalog entry.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | PK |
| `key` | string | Unique, e.g. `trial`, `pro` |
| `name` | string | |
| `status` | enum | `active` \| `archived` |

### Entitlement

Global catalog of what an **organisation** may use given its plan (product capabilities). Not the same as Permission.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | PK |
| `key` | string | Unique, e.g. `sso`, `api_access` |
| `description` | string | |

### PlanEntitlement

Join: Plan ↔ Entitlement.

| Field | Type |
| --- | --- |
| `plan_id` | FK → Plan |
| `entitlement_id` | FK → Entitlement |

### Subscription

Attaches a plan to an organisation. One active subscription per organisation in v1.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | PK |
| `organisation_id` | string | FK → Organisation |
| `plan_id` | string | FK → Plan |
| `status` | enum | `trialing` \| `active` \| `past_due` \| `canceled` |
| `current_period_end` | timestamptz | Nullable |

### OrganisationEntitlement

Per-org overrides on top of the plan.

| Field | Type | Notes |
| --- | --- | --- |
| `organisation_id` | string | FK → Organisation |
| `entitlement_id` | string | FK → Entitlement |
| `effect` | enum | `grant` \| `deny` |

**Effective entitlements** = plan entitlements ∪ grants − denies.

---

## Permission vs Entitlement

| | Permission | Entitlement |
| --- | --- | --- |
| Subject | User (via role) | Organisation (via plan) |
| Answers | “May Ada invite members?” | “May Acme use SSO?” |
| Example | `members:invite` | `sso`, `api_access` |

Product services often need both:

```text
allowed = org_has_entitlement("sso") && user_has_permission("roles:manage")
```

---

## Out of scope for v1

- Separate Account entity (billing ≠ operating org)
- Object-level ACL / ABAC
- Org-defined custom permissions (catalog is system-seeded)
- CRM pipeline / deals
