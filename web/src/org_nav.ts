export type OrgSection =
  | 'overview'
  | 'members'
  | 'roles'
  | 'users'
  | 'attributes'
  | 'email-templates'
  | 'billing'

export const ORG_SECTIONS: { id: OrgSection; label: string; path: string }[] = [
  { id: 'overview', label: 'Overview', path: '' },
  { id: 'members', label: 'Members', path: 'members' },
  { id: 'roles', label: 'Roles', path: 'roles' },
  { id: 'users', label: 'Users', path: 'users' },
  { id: 'attributes', label: 'User Attributes', path: 'attributes' },
  { id: 'email-templates', label: 'Email templates', path: 'email-templates' },
  { id: 'billing', label: 'Billing', path: 'billing' },
]

export function sectionFromParam(section?: string): OrgSection {
  switch (section) {
    case 'members':
    case 'roles':
    case 'users':
    case 'attributes':
    case 'email-templates':
    case 'billing':
      return section
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
