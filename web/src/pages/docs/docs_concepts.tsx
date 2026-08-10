import { Link, useParams } from 'react-router-dom'
import { ORG_NAV_GROUPS, type OrgSection, navLeafItems } from '../../org_nav'
import { docsBasePath } from '../../templates/paths'

export type DocConcept = {
  slug: string
  title: string
  summary: string
  body: string[]
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
    related: [{ label: 'User attributes', slug: 'attributes' }, { label: 'Variables', path: 'variables' }],
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
    slug: 'permissions-entitlements',
    title: 'Permissions vs entitlements',
    summary: 'RBAC gates operators in KYC; entitlements gate org capabilities and customer features.',
    body: [
      'Permissions (for example members:write) decide what a logged-in operator or scoped API key may do inside KYC for an organisation.',
      'Platform capabilities come from the KYC billing plan (what the org may use in KYC). Product features come from product plans (what the org unlocks for its customers). Never trust the browser alone — enforce checks in your API.',
    ],
    related: [
      { label: 'Features', slug: 'product-features' },
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
    const items = navLeafItems(group?.items ?? [])
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

  const core = ['organisation', 'permissions-entitlements'].map((slug) => {
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
