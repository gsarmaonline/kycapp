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
      'Authorisation you run for your own customers. You declare the vocabulary, KYC holds who has what, and a question is answered by walking the facts.',
    body: [
      'Members and permissions govern who may operate KYC. Customer access is the other side: which of your customers may do what inside your product, such as the project a person can edit or the region an account may read.',
      'The work is split because the knowledge is split. KYC holds your customers, your roles, your groups and your grants. It does not know what a project of yours is, which documents exist, which project a document sits in, or who owns one. A scope id is a string it stores and never resolves. So neither side can answer a real question alone: you supply the facts about your resources, KYC supplies the facts about who holds what, and the answer comes from both.',
      'Everything on both sides is the same kind of fact, written as "A’s relation is B". A grant is project:apollo #can_read app_user:ana. Containment is document:d1 #parent project:apollo. Membership is group:eng #member_of app_user:ana. There is no second mechanism hiding behind any of them, which is why one walk can answer every question.',
      'Facts only ever add. Nothing subtracts, so the order you write them in cannot change an answer, and there is no precedence to reason about. The cost of that is real and worth knowing early: there is no exception list, and no way to narrow a grant after the fact. To keep somebody out of something, grant nothing that reaches it.',
      'You can consume this two ways, and both answer from the same facts. Cache a customer’s assembled grant set and decide locally, which is the default because it keeps KYC out of your request path. Or ask a question directly with check, which is the only way to ask about a specific resource, because a grant set knows nothing about your documents.',
    ],
    steps: [
      {
        title: 'Declare your scope kinds',
        detail:
          'The levels your product has, such as project or region. You register the kind; the ids stay in your system and KYC never resolves them.',
      },
      {
        title: 'Declare your capabilities',
        detail:
          'The verbs your backend checks, written resource:action. A role can only use capabilities declared here, so a typo is refused when you write it rather than silently granting nothing when it matters.',
      },
      {
        title: 'Compose roles',
        detail:
          'Name a set of capabilities. A role may build on other roles. A grant points at the role rather than copying what it contains, so changing the role changes what every holder can do without rewriting a single grant.',
      },
      {
        title: 'Group customers, if it helps',
        detail:
          'A group is a set of customers you grant to once instead of one at a time. Groups nest, and a member of a child counts as a member of every parent, so a grant on the parent reaches them.',
      },
      {
        title: 'Issue grants',
        detail:
          'A grant gives one subject, being a customer, a group, or everyone, one set of capabilities over one scope, optionally with an expiry. An expired grant becomes invisible on its own, with no job needing to run.',
      },
      {
        title: 'Write the facts only you have',
        detail:
          'Containment, and ownership where you need it: document:d1 #parent project:apollo. Do this as resources are created. Until a document says where it lives, nothing can reach it, because there is no path to walk.',
      },
      {
        title: 'Ask',
        detail:
          'Fetch the grant set and cache it against the version, or call check for a specific resource. The version moves whenever anything changes what a customer holds, revocations included, so a cache that watches it cannot serve access that was taken away.',
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
      "all_capabilities": false
    }
  ]
}`,
    },
    examples: [
      {
        title: 'Two kinds of people',
        problem:
          'Almost every confusion here comes from one word doing two jobs. A "user" can mean somebody on your team who signs into KYC, or somebody who uses the product you sell. They are different objects with different lifecycles, and they never mix.',
        grant: `MEMBERS                        APP USERS
your team                      your customers

sign into KYC                  never see KYC
belong to your organisation    belong to your organisation
hold a KYC role                hold an app role
  owner, admin, member           whatever you name

"may Priya invite a           "may Ana edit
 teammate?"                     document d1?"

governed by Members            governed by Customer access
enforced by KYC                enforced by your backend`,
        note:
          'App users are records, not logins. They may come from your own database, or from Clerk or Auth0 by ingest, and KYC stores a profile and an id for each. Nothing you do in customer access can affect who may operate KYC, and nothing you do in Members can affect what your customers may do.',
      },
      {
        title: 'Two kinds of capability',
        problem:
          'The same split runs through what may be done, and the word "capability" is similarly overloaded. One set is fixed and belongs to KYC. The other is yours and starts empty.',
        grant: `PLATFORM                       APP
what your organisation         what your customers
may use inside KYC             may do inside your product

fixed, defined by KYC          open, declared by you
app_users:write                document:read
billing:read                   invoice:refund
members:invite                 project:archive

comes with your KYC plan       you declare it, nothing seeded
checked by KYC's gates         checked by your backend`,
        note:
          'A new organisation starts with an empty app vocabulary on purpose. A default shipped by KYC would be a guess about a product it has never seen, and a wrong guess is something people work around rather than delete. Templates offer a starting point you can read first and then edit.',
      },
      {
        title: 'And a third thing that is not access at all',
        problem:
          'Entitlements are often mistaken for permissions because both end in a yes or no. They answer a different question, and the difference shows up in what you tell the person who was refused.',
        grant: `PERMISSION            may this SUBJECT do this?
                      no  ->  "ask your admin"

ENTITLEMENT           did this ORGANISATION buy this?
                      no  ->  "upgrade your plan"`,
        note:
          'Product features and plans are entitlements: what you have unlocked for a customer commercially. Customer access is permission: what a customer is allowed to do with what they have. A customer can be entitled to a feature and not permitted to use it, and both checks have to pass.',
      },
      {
        title: 'Your first hour, end to end',
        problem:
          'A document product. Projects hold documents, editors can write, viewers can read, and everyone in a workspace can read. Here is the whole setup, in the order the pieces depend on each other, ending with a question that returns a real answer.',
        grant: `1  vocabulary          the nouns and verbs of your product
   POST /app-scope-types      { "kind": "project" }
   POST /app-capabilities     { "key": "document:read" }
   POST /app-capabilities     { "key": "document:write" }

2  roles               names for sets of capabilities
   POST /app-roles  { "key": "viewer",
                      "capabilities": ["document:read"] }
   POST /app-roles  { "key": "editor",
                      "capabilities": ["document:write"],
                      "extends": ["<viewer id>"] }

3  grants              who holds what, and where
   POST /app-grants { "app_user_id": "<ana>",
                      "role_id": "<editor id>",
                      "scope_kind": "project",
                      "scope_id": "apollo" }

4  your facts          run this from your own create path
   POST /edges { "edges": [{
     "object_type": "document", "object_id": "d1",
     "relation": "parent",
     "subject_type": "project", "subject_id": "apollo" }] }

5  ask
   POST /check { "subject_id": "ana", "action": "write",
                 "resource_type": "document", "resource_id": "d1" }
   -> { "allowed": true, "path": [ ... ] }`,
        note:
          'Steps 1 to 3 are things a person does once, in the UI or by API. Step 4 is the one that becomes code: it belongs wherever you create a document, next to the insert. Skip it and step 5 answers false no matter how many grants exist, because there is no path from the document to anything. If a check surprises you, read the path it returns: it shows the route it took, or the absence of one.',
      },
      {
        title: 'How a check actually resolves',
        problem:
          'Ana is in the engineering group, that group holds the editor role on project apollo, and document d1 lives in apollo. Nothing anywhere names Ana and d1 together. She can still edit it, and this is the walk that finds out.',
        grant: `ask:  may app_user:ana  write  document:d1 ?

document:d1  #parent     project:apollo     you wrote this
project:apollo #can_write role:editor#holder a grant
role:editor  #holder     group:eng#member_of a grant to a group
group:eng    #member_of  app_user:ana        membership

1  does d1 name ana directly?            no
2  d1 sits inside apollo, so ask apollo
3  apollo grants write to editor holders
4  editor is held by whoever is in eng
5  eng contains ana                       allowed`,
        note:
          'Four facts written by four different people at four different times, and no single one of them says Ana may edit d1. The answer is the path between them. This is also why containment is your job: remove step 1 and there is nowhere for the walk to go, however many grants exist.',
      },
      {
        title: 'Why editing a role reaches everyone at once',
        problem:
          'You add docs:delete to the editor role. Nobody rewrites any grants, and every holder gains it immediately, including customers who were granted the role months ago.',
        grant: `project:apollo #can_delete role:editor#holder

the grant names the ROLE, not what the role contains
so the walk resolves the role when the question is asked`,
        note:
          'A grant is a pointer, not a copy. The same is true of groups: adding somebody to eng gives them everything eng has been granted, without touching a grant. It cuts the other way too, so narrowing a role removes access from every holder at once, which is usually what you want and occasionally a surprise.',
      },
      {
        title: 'Three wildcards, and they compose',
        problem:
          'Wildcards exist because some sets cannot be listed: every customer includes the ones who sign up tomorrow, and every capability includes the ones you declare next quarter.',
        grant: `app_user:*   every customer, present and future
project:*    every project of that kind
can_all      every capability, now and later

project:*  #can_all  app_user:*
   every customer, every project, every capability`,
        note:
          'The capability wildcard is the one that keeps working as your vocabulary grows: declare a new capability and existing wildcard grants already carry it. That is the point of it, and the reason to be deliberate about issuing one. If a verb should never ride along, declare it on a type those grants do not reach.',
      },
      {
        title: 'Their own rows',
        problem:
          'Every customer may edit their own profile and nobody else’s. This is the most common rule in any product, and it cannot be a grant: KYC does not know your profiles exist, let alone whose is whose.',
        grant: `profile:p_ana  #owner  app_user:ana

written when the profile is created`,
        note:
          'One fact per resource, rather than one grant for everybody. Two things to know. You write it in the same place you write containment, as part of creating the resource. And owning a row confers every action its type answers, so if owners should not delete, declare delete on a type owners do not reach rather than trying to trim it here.',
      },
    ],
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
    summary: 'One subject, one set of capabilities, one scope — with wildcards where a list will not do.',
    body: [
      'A grant is the only thing that actually gives access. Everything else is vocabulary: a grant binds a subject to a set of capabilities over a scope, such as the editor role over project apollo.',
      'The subject is one customer, one group, or everyone — every customer you have, including ones who sign up later, from a single row. The capabilities come from a role, or from the wildcard: everything in your namespace, including capabilities you declare later.',
      'Grants only ever add. No grant can take away what another gives, which is what keeps the order of your grants irrelevant, and it is why there is no exception list: a rule that subtracted would have to subtract from every grant, not just its own, and then order would start to matter.',
      'The consequence is worth stating plainly. To keep somebody out of something, do not narrow a grant — issue nothing that reaches it. If you need most of a group but not everyone, the group is the place to say so.',
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
          'Write an owner edge when you create the resource: document:d1 #owner app_user:ana. The walk answers "is this thing yours?" with the comparison it already makes for every other principal. Note that owning a row confers every action its type answers, so withhold a verb by declaring it on a type owners do not reach.',
      },
      {
        title: 'Every capability',
        detail:
          'The wildcard carries capabilities you have not declared yet. That is the point: a new capability reaches every holder without anyone editing the grant, so an administrator stays an administrator. If some verb should not work that way, declare it on a type those holders do not reach.',
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
      'Every recipe below is a subject, a set of capabilities and a scope. Read them as a menu: find the sentence you are trying to say, and copy the grant beside it.',
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
        grant: `profile:p_ana  #owner  app_user:ana

written when the profile is created`,
        note: 'One fact per resource rather than one grant for everybody. KYC cannot derive it — a scope id is an opaque string it never resolves, so it has no idea which rows exist, let alone who owns them. Owning a row confers every action its type answers; to withhold one, declare it on a type owners do not reach.',
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
        title: 'Everyone but a few',
        problem:
          'A customer is being offboarded, or one account is under investigation. You want them out of the baseline.',
        grant: `subject: group / active_customers
role:    starter
scope:   tenant / acme`,
        note: 'Grant to a group rather than to everyone, and take the person out of the group. There is no exception list: a grant that subtracted would have to subtract from every other grant too, and then the order of your grants would start to matter.',
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
        title: 'Everything your product supports',
        problem:
          'An internal tools account should do whatever your product supports, now and later.',
        grant: `subject: tools@acme.com
carries: every capability
scope:   tenant / acme`,
        note: 'A wildcard carries capabilities you have not declared yet. That is the point: declare billing:refund next quarter and this grant carries it, with nobody editing anything, so an administrator stays an administrator. If a verb should never work that way, declare it on a type this grant does not reach.',
      },
      {
        title: 'Everywhere but one place',
        problem:
          'Everyone can read across the organisation, apart from the folder holding salary reviews.',
        grant: `subject: every customer
role:    reader
scope:   project / *`,
        note: 'Grant at the level you mean rather than at the ceiling, and leave the salaries project out of it. A grant cannot be narrowed after the fact, so a hard "nobody reaches this" means issuing nothing that reaches it — which is also the only kind of lock that actually holds.',
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

export function conceptNavGroups(base: string) {
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
