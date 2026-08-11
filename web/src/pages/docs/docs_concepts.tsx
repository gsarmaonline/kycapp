import { Link, useParams } from 'react-router-dom'
import { ORG_NAV_GROUPS, type OrgSection } from '../../org_nav'
import { docsBasePath } from '../../templates/paths'

export type DocConcept = {
  slug: string
  title: string
  summary: string
  body: string[]
  /** An ordered flow, for concepts where the sequence is the explanation. */
  steps?: { title: string; detail: string }[]
  /** One request or response worth showing verbatim. */
  sample?: { label: string; code: string }
  related?: { label: string; slug?: string; path?: string }[]
  section?: OrgSection
}

/** Conceptual docs for workspace elements and core platform ideas. */
export const DOC_CONCEPTS: DocConcept[] = [
  {
    slug: 'organisation',
    title: 'Organisation',
    summary: 'The tenant hub. Members, customers, packaging, and billing hang off this record.',
    body: [
      'An organisation is the merchant workspace in KYC. Everything you configure — app users, features, automations, API keys — is scoped to one organisation.',
      'Operators belong to organisations through memberships. End customers of the merchant product are app users on the organisation, not KYC login users.',
    ],
    section: 'overview',
  },
  {
    slug: 'members',
    title: 'Members',
    summary: 'People who sign into KYC and act inside an organisation via a role.',
    body: [
      'Members are KYC operators (your teammates). They authenticate with Google (or local dev login) and access only organisations they belong to.',
      'Each membership carries a role (owner, admin, member by default). Roles grant permissions such as app_users:write or billing:read — what someone may do in the KYC UI and APIs.',
    ],
    section: 'members',
  },
  {
    slug: 'users',
    title: 'Users (app users)',
    summary: 'Org-scoped end customers of the merchant product — profiles KYC stores or projects.',
    body: [
      'App users are your product’s customers, not people logging into KYC. They have email, display name, status, optional external_id, and a map of attributes.',
      'Authority can be kyc (create/edit in KYC) or external (ingest from Clerk, Auth0, your DB). Ingest upserts by external_id or email and can discover new attribute keys.',
    ],
    section: 'users',
    related: [
      { label: 'User attributes', slug: 'attributes' },
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Variables', path: 'variables' },
    ],
  },
  {
    slug: 'attributes',
    title: 'User attributes',
    summary: 'Typed profile field definitions grouped into sections for forms and ingest.',
    body: [
      'Attribute definitions describe the schema for app-user profile fields (phone, country, custom keys). Each definition has a type, optional section for UI grouping, and PII flags.',
      'When ingest runs in discover mode, unknown keys can create definitions automatically under an ingested section. Strict mode rejects unknown keys.',
    ],
    section: 'attributes',
  },
  {
    slug: 'product-features',
    title: 'Features',
    summary: 'Product capabilities you unlock for your customers, gated by plans and checks.',
    body: [
      'Product features are keys your backend asks about (for example premium_reports). They are distinct from platform capabilities, which gate what the organisation may use inside KYC itself.',
      'Features can use percentage rollout and per-subject overrides. Your merchant backend calls entitlements.check with an optional subject_id (often the Clerk user id / app user external_id).',
    ],
    section: 'product-features',
    related: [
      { label: 'Plans', slug: 'product-plans' },
      { label: 'Integration API', path: 'api' },
    ],
  },
  {
    slug: 'product-plans',
    title: 'Plans',
    summary: 'Bundles of product features (and optional Stripe prices) assigned to the organisation.',
    body: [
      'Product plans package feature keys for your customers. Activating a plan on the organisation entitles those features for entitlement checks.',
      'When Stripe is connected, plan prices can sync to Stripe Products/Prices for Checkout. KYC still owns whether access is allowed after billing events.',
    ],
    section: 'product-plans',
  },
  {
    slug: 'automations',
    title: 'Automations',
    summary: 'Rules that react to events — email, webhooks, database writes, and more.',
    body: [
      'Automations listen for triggers such as app_user.created, evaluate conditions on shared field paths, and run an ordered list of actions.',
      'Actions can send branded email, call outbound webhooks, insert into connected databases, or chain other steps. Runs are processed asynchronously by the worker.',
    ],
    section: 'automations',
    related: [
      { label: 'Emails', slug: 'email-templates' },
      { label: 'Variables', path: 'variables' },
    ],
  },
  {
    slug: 'branding',
    title: 'Branding',
    summary: 'Logo, colours, and email chrome applied when messages are rendered.',
    body: [
      'Organisation branding covers logo upload, primary/accent colours, email footer, fonts, and typography regions. Templates stay content-focused; chrome wraps at render time.',
      'Public logo URLs are used by email clients. Set PUBLIC_BASE_URL in production so assets resolve outside your private network.',
    ],
    section: 'branding',
  },
  {
    slug: 'billing',
    title: 'Billing',
    summary: 'Stripe executes Checkout and Portal; KYC owns subscription and entitlement state.',
    body: [
      'KYC is not a payment processor. Stripe is the executor for Checkout sessions, customer portal, and webhooks that update the organisation subscription.',
      'Platform capabilities come from the KYC plan on the organisation. Product features come from the active product plan. Entitlement checks read that combined state.',
    ],
    section: 'billing',
  },
  {
    slug: 'email-templates',
    title: 'Emails',
    summary: 'Org-scoped message copy with {{path}} placeholders for automations and previews.',
    body: [
      'Email templates have keys (system seeds plus custom), subjects, and body sections. Placeholders use the shared variable vocabulary (for example {{app_user.email}}).',
      'From name/address can be set per organisation or template, falling back to the platform EMAIL_FROM. Delivery uses Resend when configured.',
    ],
    section: 'email-templates',
    related: [{ label: 'Variables', path: 'variables' }],
  },
  {
    slug: 'databases',
    title: 'Databases',
    summary: 'Connected Postgres destinations for automation db_insert actions.',
    body: [
      'Database connections store host credentials for writing rows from automations. Field mappings use bare paths such as app_user.email.',
      'Connections are probed on save. Treat credentials as secrets — they are stored for the worker and returned only as masked hints in the UI.',
    ],
    section: 'databases',
  },
  {
    slug: 'webhooks',
    title: 'Outbound webhooks',
    summary: 'HTTP callbacks your automations POST to with templated JSON bodies.',
    body: [
      'Outbound webhooks define a URL, optional shared secret header, and a JSON body template using {{path}} placeholders.',
      'Automations can fire a webhook as an action. Failures are recorded on the automation run for debugging.',
    ],
    section: 'webhooks',
    related: [{ label: 'Variables', path: 'variables' }],
  },
  {
    slug: 'inbound-webhooks',
    title: 'Inbound webhooks',
    summary: 'Public endpoints that accept events from external systems into KYC automations.',
    body: [
      'Inbound webhooks give you a URL KYC listens on. Auth modes include header secret, query token, or path token.',
      'A successful receive can trigger automations so third-party systems (billing, support, ETL) drive the same lifecycle rules as native app-user events.',
    ],
    section: 'inbound-webhooks',
  },
  {
    slug: 'settings',
    title: 'Settings',
    summary: 'Organisation configuration: app-user authority, ingest rules, and integrations.',
    body: [
      'Settings control how customer profiles are owned (kyc vs external), which key ingest upserts on, attribute discover vs strict mode, and connected integrations such as Stripe.',
      'Use external authority when Clerk or another IdP is the source of truth and KYC should project customers via ingest.',
    ],
    section: 'settings',
  },
  {
    slug: 'api-keys',
    title: 'API keys',
    summary: 'Org-scoped Bearer tokens for merchant backends calling the Integration API.',
    body: [
      'Create keys under API keys. Send Authorization: Bearer kyc_… on /v1 requests. Keys may be scoped to permission keys; empty scopes mean full org access allowed by the key’s entitlements.',
      'The organisation needs the api_access platform capability. Prefer Integration API docs over the full Operator OpenAPI for merchant backends.',
    ],
    section: 'api-keys',
    related: [{ label: 'Integration API', path: 'api' }],
  },
  {
    slug: 'activity',
    title: 'Activity',
    summary: 'Recent mutating actions and usage visibility for the organisation.',
    body: [
      'Activity surfaces recent events in the workspace — useful for support and auditing what operators and automations did.',
      'Usage metering for entitlement checks can appear in overview charts when the observability database is configured.',
    ],
    section: 'activity',
  },
  {
    slug: 'customer-access',
    title: 'Customer access',
    summary:
      'Authorisation you run for your own customers. You declare the vocabulary, KYC stores the grants, your backend decides.',
    body: [
      'Members and permissions govern who may operate KYC. Customer access is the other side: which of your customers may do what inside your product — the project a person can edit, the region an account may read.',
      'KYC does not enforce this. It has no idea what a project of yours is, and asking it on every request would put a network hop inside your app and make KYC own your latency. Instead you declare the vocabulary here, grant roles over your own scopes, and read back an assembled grant set that your backend evaluates locally.',
      'Because the vocabulary is yours, the capability set is open: anything you declare is valid. KYC keeps it inside your organisation and can never be used to grant power inside KYC itself. The two namespaces never mix.',
    ],
    steps: [
      {
        title: 'Declare your scope kinds',
        detail:
          'The levels your product has, such as project or region. You register the kind; the ids stay in your system.',
      },
      {
        title: 'Declare your capabilities',
        detail:
          'The verbs your backend checks, written resource:action. A role can only use capabilities declared here, so a typo is caught rather than silently granting nothing.',
      },
      {
        title: 'Compose roles',
        detail:
          'Name a set of capabilities. A role may build on other roles, and what it resolves to is recomputed when you change it, so every holder follows.',
      },
      {
        title: 'Group customers, if it helps',
        detail:
          'A group is a set of customers you grant to once instead of one at a time. Membership is an explicit list you manage on the group.',
      },
      {
        title: 'Issue grants',
        detail:
          'A grant gives one subject — a customer or a group — one role over one scope, optionally with an expiry.',
      },
      {
        title: 'Read the grant set back',
        detail:
          'Your backend fetches a customer’s assembled access, caches it against the version, and decides locally.',
      },
    ],
    sample: {
      label: 'Reading a customer’s access',
      code: `GET /v1/app-users/{id}/access

{
  "app_user_id": "01J…",
  "namespace": "org:01J…",
  "version": 1786374167,
  "grants": [
    {
      "scope_kind": "project",
      "scope_id": "apollo",
      "capabilities": ["docs:read", "docs:write"],
      "source": "group:au_customers app-role:editor"
    }
  ]
}`,
    },
    related: [
      { label: 'Scope kinds', slug: 'customer-scope-kinds' },
      { label: 'Capabilities', slug: 'customer-capabilities' },
      { label: 'Roles', slug: 'customer-roles' },
      { label: 'Groups', slug: 'customer-groups' },
      { label: 'Grants', slug: 'customer-grants' },
      { label: 'Users (app users)', slug: 'users' },
      { label: 'Permissions vs entitlements', slug: 'permissions-entitlements' },
      { label: 'Integration API', path: 'api' },
    ],
  },
  {
    slug: 'customer-scope-kinds',
    title: 'Scope kinds',
    summary: 'The levels your product has — project, region, account — that access can be granted over.',
    body: [
      'A scope kind names a level in your product. You register the kind once; the ids underneath it stay in your system and are never uploaded. A grant then reads as a role over project apollo, where project is the kind and apollo is your id.',
      'Kinds are flat and independent, not a tree. If a resource belongs to several containers at once, list all of them when you check: an object in two projects is reachable through either, and adding it to a third only ever widens access.',
    ],
    section: 'customer-scope-kinds',
    related: [
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Grants', slug: 'customer-grants' },
    ],
  },
  {
    slug: 'customer-capabilities',
    title: 'Capabilities',
    summary: 'The verbs your backend checks, written resource:action.',
    body: [
      'A capability is one thing a customer may do, such as invoices:read or docs:write. Declare the ones your product actually checks; they are yours, and KYC only stores them.',
      'A role can use nothing that is not declared here. That is the point of declaring them: a mistyped capability is rejected when you build the role, instead of quietly granting nothing at the moment it matters.',
    ],
    section: 'customer-capabilities',
    related: [
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Roles', slug: 'customer-roles' },
    ],
  },
  {
    slug: 'customer-roles',
    title: 'Roles',
    summary: 'Named sets of capabilities, which may build on other roles.',
    body: [
      'A role is a capability set with a name. Grants carry roles rather than raw capabilities, so you can change what maintainer means in one place and everyone holding it follows.',
      'A role may build on others. What it resolves to is worked out and stored when you save it, so a check never has to walk a chain, and editing a base role updates everything built on it. The detail page shows which capabilities are the role’s own and which are inherited.',
      'Roles never subtract. Building on a role can only add capabilities, which is what keeps the resolved set predictable no matter how deep the chain runs.',
    ],
    section: 'customer-roles',
    related: [
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Capabilities', slug: 'customer-capabilities' },
      { label: 'Grants', slug: 'customer-grants' },
    ],
  },
  {
    slug: 'customer-groups',
    title: 'Groups',
    summary: 'Sets of customers you grant to once instead of one at a time.',
    body: [
      'A group is a named set of your customers. Granting a role to a group reaches every member, so onboarding a person becomes adding them to a group rather than reissuing their grants.',
      'Membership is an explicit list, managed on the group’s own page. It is not a query over attributes: a rule that recomputes silently would change who has access without anyone issuing anything, and the reason for that change would be invisible on the day it mattered.',
      'A customer keeps whatever they hold directly as well. Group access and direct access add together, and a customer’s page shows which grant came through which group.',
    ],
    section: 'customer-groups',
    related: [
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Grants', slug: 'customer-grants' },
      { label: 'Users (app users)', slug: 'users' },
    ],
  },
  {
    slug: 'customer-grants',
    title: 'Grants',
    summary: 'One subject, one role, one scope, optionally until a date.',
    body: [
      'A grant is the only thing that actually gives access. Everything else is vocabulary: a grant binds a subject — one customer or one group — to a role over a scope, such as the editor role over project apollo.',
      'Grants are issued and revoked, never edited. Changing one in place would rewrite what someone had at a past moment; revoking and issuing leaves a history you can read. Set an expiry when access should end on its own.',
      'Access is additive and denied by default. There are no deny rules, so no grant can take away what another gives, and a customer with no grant for a scope cannot tell it apart from a scope that does not exist.',
    ],
    section: 'customer-grants',
    related: [
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Roles', slug: 'customer-roles' },
      { label: 'Groups', slug: 'customer-groups' },
    ],
  },
  {
    slug: 'permissions-entitlements',
    title: 'Permissions vs entitlements',
    summary: 'RBAC gates operators in KYC; entitlements gate org capabilities and customer features.',
    body: [
      'Permissions (for example members:write) decide what a logged-in operator or scoped API key may do inside KYC for an organisation.',
      'Platform capabilities come from the KYC billing plan (what the org may use in KYC). Product features come from product plans (what the org unlocks for its customers). Never trust the browser alone — enforce checks in your API.',
    ],
    related: [
      { label: 'Features', slug: 'product-features' },
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Integration API', path: 'api' },
    ],
  },
]

const BY_SLUG = new Map(DOC_CONCEPTS.map((c) => [c.slug, c]))

export function getDocConcept(slug: string): DocConcept | undefined {
  return BY_SLUG.get(slug)
}

function relatedHref(base: string, related: { slug?: string; path?: string }): string {
  if (related.slug) return `${base}/concepts/${related.slug}`
  if (related.path) return `${base}/${related.path}`
  return base
}

function conceptNavGroups(base: string) {
  const bySection = new Map(
    DOC_CONCEPTS.filter((c) => c.section).map((c) => [c.section!, c]),
  )

  const fromGroup = (groupId: string, label: string, hint: string) => {
    const group = ORG_NAV_GROUPS.find((g) => g.id === groupId)
    const items = (group?.items ?? [])
      .map((item) => bySection.get(item.id))
      .filter((c): c is DocConcept => Boolean(c))
      .map((c) => ({
        slug: c.slug,
        title: c.title,
        summary: c.summary,
        href: `${base}/concepts/${c.slug}`,
      }))
    return { id: groupId, label, hint, items }
  }

  const core = ['organisation', 'permissions-entitlements', 'customer-access'].map((slug) => {
    const c = BY_SLUG.get(slug)!
    return {
      slug: c.slug,
      title: c.title,
      summary: c.summary,
      href: `${base}/concepts/${c.slug}`,
    }
  })

  return [
    {
      id: 'core',
      label: 'Core ideas',
      hint: 'How KYC models tenants and access',
      items: core,
    },
    fromGroup('product', 'Product', 'What the organisation runs for its customers'),
    fromGroup(
      'customer-access',
      'Customer access',
      'What the organisation’s own customers may do inside its product',
    ),
    fromGroup('actions', 'Actions', 'Destinations and inbound triggers for automations'),
    fromGroup('platform', 'Platform', 'What the organisation uses inside KYC'),
  ]
}

export function DocsConceptsIndex() {
  const { orgId } = useParams()
  const base = docsBasePath(orgId)
  const groups = conceptNavGroups(base)

  return (
    <article className="docs-concepts prose-docs">
      <p>
        KYC is the system of record for organisations: configure product surface and lifecycle here,
        enforce access in your backend, and store (or project) customer profiles. Browse concepts
        below, then use the <Link to={`${base}/api`}>API reference</Link> when integrating.
      </p>

      {groups.map((group) => (
        <section key={group.id} className="docs-concept-group">
          <h2>{group.label}</h2>
          <p className="docs-concept-group-hint">{group.hint}</p>
          <ul className="docs-concept-list">
            {group.items.map((item) => (
              <li key={item.slug}>
                <Link to={item.href}>
                  <strong>{item.title}</strong>
                  <span>{item.summary}</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </article>
  )
}

export function DocsConceptPage() {
  const { orgId, slug = '' } = useParams()
  const base = docsBasePath(orgId)
  const concept = getDocConcept(slug)

  if (!concept) {
    return (
      <article className="docs-concepts prose-docs">
        <p>Unknown concept.</p>
        <p>
          <Link to={base}>Back to concepts</Link>
        </p>
      </article>
    )
  }

  return (
    <article className="docs-concepts prose-docs">
      <p className="docs-concept-crumb">
        <Link to={base}>Concepts</Link>
        <span aria-hidden="true"> / </span>
        <span>{concept.title}</span>
      </p>
      <h2>{concept.title}</h2>
      <p className="docs-concept-summary">{concept.summary}</p>
      {concept.body.map((para) => (
        <p key={para.slice(0, 48)}>{para}</p>
      ))}
      {concept.steps && concept.steps.length > 0 && (
        <>
          <h3>How it fits together</h3>
          <ol className="docs-concept-steps">
            {concept.steps.map((step) => (
              <li key={step.title}>
                <strong>{step.title}</strong>
                <span>{step.detail}</span>
              </li>
            ))}
          </ol>
        </>
      )}
      {concept.sample && (
        <>
          <h3>{concept.sample.label}</h3>
          <pre className="docs-concept-sample">
            <code>{concept.sample.code}</code>
          </pre>
        </>
      )}
      {concept.related && concept.related.length > 0 && (
        <>
          <h3>Related</h3>
          <ul>
            {concept.related.map((r) => (
              <li key={r.label}>
                <Link to={relatedHref(base, r)}>{r.label}</Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </article>
  )
}
