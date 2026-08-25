export type OrgSection =
  | 'overview'
  | 'members'
  | 'roles'
  | 'users'
  | 'attributes'
  // The four pages the sidebar shows.
  | 'customer-model'
  | 'customer-roles-groups'
  | 'customer-access'
  | 'customer-playground'
  // Still addressable, because every create, show and edit screen lives under
  // one of these. They are no longer nav items: each is a section of one of the
  // four above.
  | 'customer-scope-kinds'
  | 'customer-capabilities'
  | 'customer-roles'
  | 'customer-groups'
  | 'customer-grants'
  | 'customer-map'
  | 'customer-edges'
  | 'email-templates'
  | 'databases'
  | 'webhooks'
  | 'inbound-webhooks'
  | 'automations'
  | 'product-features'
  | 'product-plans'
  | 'branding'
  | 'billing'
  | 'settings'
  | 'api-keys'
  | 'activity'
  | 'docs'

export type NavGroupId = 'product' | 'customer-access' | 'actions' | 'platform'

export type OrgNavItem = { id: OrgSection; label: string; path: string }

export type OrgNavGroup = {
  id: NavGroupId
  label: string
  hint: string
  items: OrgNavItem[]
  /**
   * Open on first render. Surfaces used daily stay open; those configured once
   * and revisited rarely start collapsed, which is what keeps the sidebar
   * short without hiding anything you reach for often.
   *
   * A collapsed group still opens itself when one of its pages is current, so
   * you are never on a page that is invisible in the nav.
   */
  defaultOpen?: boolean
}

/** Flat list kept for overview tiles and path helpers. */
export const ORG_SECTIONS: OrgNavItem[] = [
  { id: 'overview', label: 'Overview', path: '' },
  { id: 'members', label: 'Members', path: 'members' },
  { id: 'users', label: 'Users', path: 'users' },
  { id: 'attributes', label: 'User Attributes', path: 'attributes' },
  { id: 'customer-model', label: 'Model', path: 'customer-model' },
  { id: 'customer-roles-groups', label: 'Roles & groups', path: 'customer-roles-groups' },
  { id: 'customer-access', label: 'Access', path: 'customer-access' },
  { id: 'customer-playground', label: 'Playground', path: 'customer-playground' },
  { id: 'email-templates', label: 'Emails', path: 'email-templates' },
  { id: 'databases', label: 'Databases', path: 'databases' },
  { id: 'webhooks', label: 'Outbound webhooks', path: 'webhooks' },
  { id: 'inbound-webhooks', label: 'Inbound webhooks', path: 'inbound-webhooks' },
  { id: 'automations', label: 'Automations', path: 'automations' },
  { id: 'product-features', label: 'Features', path: 'product-features' },
  { id: 'product-plans', label: 'Plans', path: 'product-plans' },
  { id: 'branding', label: 'Branding', path: 'branding' },
  { id: 'billing', label: 'Billing', path: 'billing' },
  { id: 'settings', label: 'Settings', path: 'settings' },
  { id: 'api-keys', label: 'API keys', path: 'api-keys' },
  { id: 'activity', label: 'Activity', path: 'activity' },
  { id: 'docs', label: 'Documentation', path: 'docs' },
]

/** Sidebar groups: product, action destinations, then KYC platform. */
export const ORG_NAV_GROUPS: OrgNavGroup[] = [
  {
    id: 'product',
    label: 'Product features',
    hint: 'What this organisation runs for its own users',
    defaultOpen: true,
    items: [
      { id: 'users', label: 'Users', path: 'users' },
      { id: 'attributes', label: 'User Attributes', path: 'attributes' },
      { id: 'automations', label: 'Automations', path: 'automations' },
      { id: 'branding', label: 'Branding', path: 'branding' },
      { id: 'product-features', label: 'Features', path: 'product-features' },
      { id: 'product-plans', label: 'Plans', path: 'product-plans' },
    ],
  },
  {
    // Its own section rather than a dropdown inside another one. Nesting it
    // would put these pages three levels deep while everything else sits at
    // two, and this surface is only going to grow.
    id: 'customer-access',
    label: 'Customer access',
    hint: "What this organisation's own customers may do inside its product",
    // Four concepts, not eight tables.
    //
    // The eight were presented as peers and were nothing like peers: a
    // vocabulary declared once, two names for one mechanism, facts a backend
    // writes constantly, and two views that are not objects at all. The order
    // followed nothing — not what you do first, not what depends on what.
    //
    // These four are in the order you do them in: define, build, grant, check.
    items: [
      { id: 'customer-model', label: 'Model', path: 'customer-model' },
      { id: 'customer-roles-groups', label: 'Roles & groups', path: 'customer-roles-groups' },
      { id: 'customer-access', label: 'Access', path: 'customer-access' },
      { id: 'customer-playground', label: 'Playground', path: 'customer-playground' },
    ],
  },
  {
    id: 'actions',
    label: 'Actions',
    hint: 'Destinations and inbound triggers used by automations',
    items: [
      { id: 'email-templates', label: 'Emails', path: 'email-templates' },
      { id: 'databases', label: 'Databases', path: 'databases' },
      { id: 'webhooks', label: 'Outbound webhooks', path: 'webhooks' },
      { id: 'inbound-webhooks', label: 'Inbound webhooks', path: 'inbound-webhooks' },
    ],
  },
  {
    id: 'platform',
    label: 'Platform capabilities',
    hint: 'What this organisation uses inside KYC',
    items: [
      { id: 'members', label: 'Members', path: 'members' },
      { id: 'settings', label: 'Settings', path: 'settings' },
      { id: 'api-keys', label: 'API keys', path: 'api-keys' },
      // This organisation's own subscription to KYC, not what it charges its
      // customers. It sat under product features, which read as the latter.
      { id: 'billing', label: 'Billing', path: 'billing' },
      { id: 'activity', label: 'Activity', path: 'activity' },
      { id: 'docs', label: 'Documentation', path: 'docs' },
    ],
  },
]
export function sectionFromPathname(pathname: string, orgId: string): OrgSection {
  const prefix = `/orgs/${orgId}`
  if (!pathname.startsWith(prefix)) return 'overview'
  const rest = pathname.slice(prefix.length).replace(/^\//, '')
  const head = rest.split('/')[0] || ''
  switch (head) {
    case 'members':
    case 'roles':
    case 'users':
    case 'attributes':
    case 'customer-model':
    case 'customer-roles-groups':
    case 'customer-access':
    case 'customer-playground':
    case 'email-templates':
    case 'databases':
    case 'webhooks':
    case 'inbound-webhooks':
    case 'automations':
    case 'product-features':
    case 'product-plans':
    case 'branding':
    case 'billing':
    case 'settings':
    case 'api-keys':
    case 'activity':
    case 'docs':
      return head
    // A create, show or edit screen keeps its own path and highlights the page
    // it belongs to. Without this you would be editing a role while the sidebar
    // showed nothing selected, which reads as having navigated out of the app.
    case 'customer-scope-kinds':
    case 'customer-capabilities':
    case 'customer-map':
      return 'customer-model'
    case 'customer-roles':
    case 'customer-groups':
      return 'customer-roles-groups'
    case 'customer-grants':
    case 'customer-edges':
      return 'customer-access'
    case 'schema':
      return 'attributes'
    default:
      return 'overview'
  }
}

export function orgPath(orgId: string, section: OrgSection = 'overview') {
  const base = `/orgs/${orgId}`
  if (section === 'overview') return base
  return `${base}/${section}`
}

export function resourcePath(
  orgId: string,
  section: Exclude<
    OrgSection,
    | 'overview'
    | 'billing'
    | 'branding'
    | 'settings'
    | 'docs'
    | 'activity'
    | 'customer-map'
    | 'customer-edges'
    | 'customer-playground'
  >,
  ...parts: string[]
) {
  const base = orgPath(orgId, section)
  if (!parts.length) return base
  return `${base}/${parts.join('/')}`
}
