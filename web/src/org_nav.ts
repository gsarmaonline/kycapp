export type OrgSection =
  | 'overview'
  | 'members'
  | 'roles'
  | 'users'
  | 'attributes'
  | 'customer-scope-kinds'
  | 'customer-capabilities'
  | 'customer-roles'
  | 'customer-groups'
  | 'customer-grants'
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

export type NavGroupId = 'product' | 'actions' | 'platform'

export type OrgNavItem = { id: OrgSection; label: string; path: string }

/**
 * A sidebar entry. Either a link to a page, or a container whose children are
 * links. A container has no page of its own, so it toggles rather than
 * navigates.
 */
export type OrgNavEntry =
  | (OrgNavItem & { children?: undefined })
  | { id: string; label: string; children: OrgNavItem[] }

export type OrgNavGroup = {
  id: NavGroupId
  label: string
  hint: string
  items: OrgNavEntry[]
}

/** Flat list kept for overview tiles and path helpers. */
export const ORG_SECTIONS: OrgNavItem[] = [
  { id: 'overview', label: 'Overview', path: '' },
  { id: 'members', label: 'Members', path: 'members' },
  { id: 'users', label: 'Users', path: 'users' },
  { id: 'attributes', label: 'User Attributes', path: 'attributes' },
  { id: 'customer-scope-kinds', label: 'Scope kinds', path: 'customer-scope-kinds' },
  { id: 'customer-capabilities', label: 'Capabilities', path: 'customer-capabilities' },
  { id: 'customer-roles', label: 'Roles', path: 'customer-roles' },
  { id: 'customer-groups', label: 'Groups', path: 'customer-groups' },
  { id: 'customer-grants', label: 'Grants', path: 'customer-grants' },
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
    items: [
      { id: 'users', label: 'Users', path: 'users' },
      { id: 'attributes', label: 'User Attributes', path: 'attributes' },
      {
        id: 'customer-access',
        label: 'Customer access',
        // A container, not a page. Each child is one object type, matching how
        // the rest of this sidebar works.
        children: [
          { id: 'customer-scope-kinds', label: 'Scope kinds', path: 'customer-scope-kinds' },
          { id: 'customer-capabilities', label: 'Capabilities', path: 'customer-capabilities' },
          { id: 'customer-roles', label: 'Roles', path: 'customer-roles' },
          { id: 'customer-groups', label: 'Groups', path: 'customer-groups' },
          { id: 'customer-grants', label: 'Grants', path: 'customer-grants' },
        ],
      },
      { id: 'automations', label: 'Automations', path: 'automations' },
      { id: 'branding', label: 'Branding', path: 'branding' },
      { id: 'product-features', label: 'Features', path: 'product-features' },
      { id: 'product-plans', label: 'Plans', path: 'product-plans' },
      { id: 'billing', label: 'Billing', path: 'billing' },
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
    case 'customer-scope-kinds':
    case 'customer-capabilities':
    case 'customer-roles':
    case 'customer-groups':
    case 'customer-grants':
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
  section: Exclude<OrgSection, 'overview' | 'billing' | 'branding' | 'settings' | 'docs' | 'activity'>,
  ...parts: string[]
) {
  const base = orgPath(orgId, section)
  if (!parts.length) return base
  return `${base}/${parts.join('/')}`
}

/**
 * The link entries of a group, with containers flattened to their children.
 * Callers that care about pages rather than sidebar shape use this.
 */
export function navLeafItems(items: OrgNavEntry[]): OrgNavItem[] {
  return items.flatMap((item) => (item.children ? item.children : [item]))
}
