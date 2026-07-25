# Variable referencing

KYC uses one shared path vocabulary for templating and field lookups across:

- Email templates (`subject`, `body_text`, `body_html`)
- Outbound webhook `body_template` JSON
- Automation conditions
- `db_insert` column mappings
- Trigger-parameter matching

## Syntax

| Surface | Form | Example |
| --- | --- | --- |
| Emails & webhook JSON strings | `{{path}}` | `{{app_user.email}}` |
| Conditions / `db_insert` mappings | bare path | `app_user.email` |

Braces are only for string templates. Paths are the same either way.

## Canonical paths

| Path | Meaning |
| --- | --- |
| `app_user.id` | App user id |
| `app_user.email` | Email |
| `app_user.display_name` | Display name |
| `app_user.status` | `active` / `disabled` / … |
| `app_user.external_id` | Merchant external id |
| `app_user.<attribute_key>` | Org attribute (e.g. `app_user.country`) |
| `organisation.id` / `organisation.name` | Organisation (injected for email render) |
| `organisation_id` | Run / webhook metadata |
| `trigger` | Event id (e.g. `app_user.created`) |

Attribute keys use the **definition key**, not `attributes.country`. Prefer `app_user.country`.

## Engines (single library)

| Language | Package / module | Entry points |
| --- | --- | --- |
| Go | `core/automations` | `Lookup`, `NormalizeFieldPath`, `RenderStringTemplate`, `RenderJSONTemplate` |
| TypeScript (web preview) | `web/src/templates/paths.ts` | `lookupPath`, `normalizeFieldPath`, `renderStringTemplate` |

Email send uses `emailtemplates.Render` → `automations.RenderStringTemplate`. Web preview uses the TypeScript module (same path rules).

## Legacy aliases

Still resolved at runtime via `NormalizeFieldPath`:

| Alias | Canonical |
| --- | --- |
| `email`, `display_name`, `status`, … | `app_user.<field>` |
| `attributes.<key>` | `app_user.<key>` |
| `org_name` | `organisation.name` |

Prefer canonical paths in new templates.

## Missing values

- String templates (`{{path}}`): missing → empty string
- JSON webhook whole-string `"{{path}}"`: missing → `null`
- Conditions: `exists` / `not_exists` ops

## Examples

Email body:

```html
<p>Hi {{app_user.display_name}},</p>
<p>Welcome to {{organisation.name}}.</p>
```

Webhook `body_template`:

```json
{
  "organisation_id": "{{organisation_id}}",
  "trigger": "{{trigger}}",
  "email": "{{app_user.email}}",
  "country": "{{app_user.country}}"
}
```

`db_insert` mapping:

```json
{ "email": "app_user.email", "country": "app_user.country" }
```

Condition:

```json
{ "field": "app_user.country", "op": "eq", "value": "AU" }
```
