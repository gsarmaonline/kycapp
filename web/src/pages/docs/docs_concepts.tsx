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
  /**
   * Worked examples: a thing someone wants to express, and the grant that
   * expresses it. `note` carries the catch, which is usually the part that
   * matters.
   */
  examples?: { title: string; problem: string; grant: string; note?: string }[]
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
      'Because your backend decides, every field of a grant matters to it. A grant may carry a wildcard, exceptions, or a self constraint, and code that reads only the capability list will allow more than you granted. KYC cannot catch that, because the check runs in your process.',
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
          'A grant gives one subject — a customer, a group, or everyone — one set of capabilities over one scope, optionally with an expiry and exceptions.',
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
      "source": "group:au_customers app-role:editor",
      "all_capabilities": false,
      "except_capabilities": [],
      "except_scopes": [{ "kind": "project", "id": "salaries" }],
      "constraint": ""
    }
  ]
}`,
    },
    related: [
      { label: 'Access recipes', slug: 'customer-access-examples' },
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
    summary: 'One subject, one set of capabilities, one scope — with wildcards and exceptions where a list will not do.',
    body: [
      'A grant is the only thing that actually gives access. Everything else is vocabulary: a grant binds a subject to a set of capabilities over a scope, such as the editor role over project apollo.',
      'The subject is one customer, one group, or everyone — every customer you have, including ones who sign up later, from a single row. The capabilities come from a role, or from the wildcard: everything in your namespace, including capabilities you declare later.',
      'A grant can narrow itself. Each wildcard has an exception list beside it: everyone except these customers, everything except account:delete, all of Acme except the salaries project. A wildcard claims a set nobody can enumerate; an exception names the members that do not belong.',
      'Every exception narrows the grant it sits on and nothing else. No grant can take away what another gives, which is what keeps the order of your grants irrelevant. The limit that follows: an exception is not a lock. If a second grant reaches an excluded resource, that grant allows, so a hard "nobody" means issuing nothing that reaches it.',
      'Grants are issued and revoked, never edited. Changing one in place would rewrite what someone had at a past moment; revoking and issuing leaves a history you can read. Set an expiry when access should end on its own.',
    ],
    steps: [
      {
        title: 'Everyone, for a baseline',
        detail:
          'One row covering every customer you have and every one you will have. The alternative is adding each person to a group as they sign up, which costs a row each and says the same thing.',
      },
      {
        title: 'Only their own resources',
        detail:
          'The self constraint applies a grant to rows belonging to the holder. Together with the everyone subject, that is "customers may manage their own things" in one grant. It says nothing about which verbs are allowed — leave account:delete out of the role for that.',
      },
      {
        title: 'Every capability, with carve-outs',
        detail:
          'The wildcard carries capabilities you have not declared yet. That is the point and also the risk: a new capability reaches every holder without anyone editing the grant. The carve-out list names the dangerous verbs you know of today.',
      },
      {
        title: 'Except these scopes',
        detail:
          'For what narrower scoping cannot say. Ten thousand projects and one confidential would need 9,999 grants to express positively; one exception says it directly.',
      },
    ],
    section: 'customer-grants',
    related: [
      { label: 'Access recipes', slug: 'customer-access-examples' },
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Roles', slug: 'customer-roles' },
      { label: 'Groups', slug: 'customer-groups' },
    ],
  },
  {
    slug: 'customer-access-examples',
    title: 'Access recipes',
    summary:
      'The common access rules, each written as the grant that expresses it — and the ones this model deliberately will not.',
    body: [
      'One model, many shapes. Role-based access, ownership, per-project access, time-boxed access and a customer-wide baseline are not different systems here. Each is a particular way of filling in a grant.',
      'Every recipe below is a subject, a set of capabilities, a scope, and sometimes a constraint or an exception. Read them as a menu: find the sentence you are trying to say, and copy the grant beside it.',
    ],
    examples: [
      {
        title: 'A role over a place',
        problem:
          'The ordinary case, and the one every other recipe varies. Priya edits documents in the Apollo project, and nowhere else.',
        grant: `subject: Priya
role:    editor          (docs:read, docs:write)
scope:   project / apollo`,
        note: 'Change what editor means and every holder follows, because the grant carries the role rather than a copy of its capabilities.',
      },
      {
        title: 'Seniority, without repeating yourself',
        problem:
          'A maintainer can do everything an editor can, plus publish. You want to say that once, not maintain two lists that drift.',
        grant: `viewer                    docs:read
editor      builds on viewer    + docs:write
maintainer  builds on editor    + docs:publish`,
        note: 'Roles only ever add. Two paths to the same capability give the same answer, so a role built on two others needs no tie-break rule.',
      },
      {
        title: 'Their own things',
        problem:
          'Every customer may read and edit their own profile, and nobody else’s. This is the most common rule in any product, and the one people reach for a special case to express.',
        grant: `subject:    every customer
role:       self_manager     (profile:read, profile:write)
scope:      tenant / acme
constraint: only their own resources`,
        note: 'The constraint answers "is this thing yours?" and nothing else. It has no opinion on which verbs are allowed: account deletion is prevented by leaving it out of the role.',
      },
      {
        title: 'A baseline for everyone, including tomorrow',
        problem:
          'Every customer should start with something, and you do not want to issue a grant each time somebody signs up.',
        grant: `subject: every customer
role:    starter
scope:   tenant / acme`,
        note: 'One row, however many customers you have. A person who signs up next year is covered without anyone touching it.',
      },
      {
        title: 'Everyone except a few',
        problem:
          'A customer is being offboarded, or one account is under investigation. You want them out of the baseline without listing everybody else.',
        grant: `subject: every customer
         except: dana@customer.com
role:    starter
scope:   tenant / acme`,
        note: 'The exception narrows this grant alone. If Dana holds another grant that reaches the same scope, that one still allows.',
      },
      {
        title: 'Someone on several projects',
        problem:
          'Priya is an editor on Apollo and a viewer on two others, and a document belongs to two projects at once.',
        grant: `grants:   (project / apollo,   editor)
          (project / borealis, viewer)
          (project / ceres,    viewer)

document: project: [apollo, ceres]`,
        note: 'Matching any one coordinate is enough. Adding the document to a fourth project can only widen who reaches it, never narrow it, so you never have to reason about ordering.',
      },
      {
        title: 'Access by attribute, without attribute rules',
        problem:
          'Everyone in Australia should get regional access. You want it to apply automatically, not by hand.',
        grant: `group: au_customers      (explicit membership)
grant: (au_customers, regional_reader, region / apac)`,
        note: 'The rule that decides membership runs in your onboarding or an automation on app_user.created. What lands here is the decision, at a known moment. Access then changes when someone changes access, not when someone edits a phone number.',
      },
      {
        title: 'Access that ends by itself',
        problem:
          'A contractor needs elevated access for a fortnight. You would rather not rely on remembering to revoke it.',
        grant: `subject: Priya
role:    incident_responder
scope:   tenant / acme
expires: 12 August 2026`,
        note: 'An expired grant is invisible rather than denied, so it reads exactly like access that was never granted.',
      },
      {
        title: 'Everything except the dangerous verb',
        problem:
          'An internal tools account should do whatever your product supports, but never delete an account.',
        grant: `subject: tools@acme.com
carries: every capability
         except: account:delete
scope:   tenant / acme`,
        note: 'A wildcard carries capabilities you have not declared yet. That is the point, and the risk: declare billing:refund next quarter and this grant carries it, with nobody editing anything. The carve-out names the verbs you know about today, not tomorrow’s.',
      },
      {
        title: 'Everything except one place',
        problem:
          'Everyone can read across the organisation, apart from the folder holding salary reviews. You have ten thousand projects and one of them is confidential.',
        grant: `subject: every customer
role:    reader
scope:   tenant / acme
         except: project / salaries`,
        note: 'Written positively this needs 9,999 grants. But an exception is not a lock: if another grant reaches the salaries project, that grant allows. For a hard "nobody", issue nothing that reaches it.',
      },
    ],
    related: [
      { label: 'Customer access', slug: 'customer-access' },
      { label: 'Grants', slug: 'customer-grants' },
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

  const asItem = (c: DocConcept) => ({
    slug: c.slug,
    title: c.title,
    summary: c.summary,
    href: `${base}/concepts/${c.slug}`,
  })

  // extraSlugs carries concepts with no workspace page of their own, such as
  // the recipes, which belong in a group without being an object you can open.
  const fromGroup = (groupId: string, label: string, hint: string, extraSlugs: string[] = []) => {
    const group = ORG_NAV_GROUPS.find((g) => g.id === groupId)
    const items = (group?.items ?? [])
      .map((item) => bySection.get(item.id))
      .filter((c): c is DocConcept => Boolean(c))
      .map(asItem)
    for (const slug of extraSlugs) {
      const c = BY_SLUG.get(slug)
      if (c) items.push(asItem(c))
    }
    return { id: groupId, label, hint, items }
  }

  const core = ['organisation', 'permissions-entitlements', 'customer-access'].map((slug) =>
    asItem(BY_SLUG.get(slug)!),
  )

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
      ['customer-access-examples'],
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
          {/*
            The heading and its hint sit in a banded header rather than floating
            above the list. Spacing alone did not separate one group from the
            next: every row already carried a rule, so a group boundary looked
            exactly like a row divider.
          */}
          <header className="docs-concept-group-head">
            <h2>{group.label}</h2>
            <p className="docs-concept-group-hint">{group.hint}</p>
            <span className="docs-concept-count">
              {group.items.length} {group.items.length === 1 ? 'concept' : 'concepts'}
            </span>
          </header>
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
      {concept.examples?.map((ex) => (
        <section className="docs-example" key={ex.title}>
          <h3>{ex.title}</h3>
          <p>{ex.problem}</p>
          <pre className="docs-concept-sample">
            <code>{ex.grant}</code>
          </pre>
          {ex.note && <p className="docs-example-note">{ex.note}</p>}
        </section>
      ))}
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
