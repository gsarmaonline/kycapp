import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import App from './App'

const me = vi.fn()
const listOrganisations = vi.fn()
const authProviders = vi.fn()
const getOrganisation = vi.fn()
const listMemberships = vi.fn()
const listRoles = vi.fn()
const listPermissions = vi.fn()

vi.mock('./api', () => ({
  getToken: vi.fn(() => 'test-token'),
  setToken: vi.fn(),
  captureOAuthTokenFromHash: vi.fn(() => false),
  me: (...args: unknown[]) => me(...args),
  logout: vi.fn(),
  authProviders: (...args: unknown[]) => authProviders(...args),
  googleAuthURL: vi.fn(() => '/v1/auth/google'),
  devLogin: vi.fn(),
  listOrganisations: (...args: unknown[]) => listOrganisations(...args),
  createOrganisation: vi.fn(),
  getOrganisation: (...args: unknown[]) => getOrganisation(...args),
  listMemberships: (...args: unknown[]) => listMemberships(...args),
  listRoles: (...args: unknown[]) => listRoles(...args),
  listPermissions: (...args: unknown[]) => listPermissions(...args),
  inviteMember: vi.fn(),
  updateRole: vi.fn(),
  createRole: vi.fn(),
  listPlans: vi.fn(async () => ({ items: [] })),
  listEntitlementsCatalog: vi.fn(async () => ({ items: [] })),
  getSubscription: vi.fn(),
  getOrgEntitlements: vi.fn(async () => ({ entitlements: [] })),
  listAttributeDefinitions: vi.fn(async () => ({ items: [] })),
  createAttributeDefinition: vi.fn(),
  listAppUsers: vi.fn(async () => ({ items: [] })),
  createAppUser: vi.fn(),
  listEmailTemplates: vi.fn(async () => ({ items: [] })),
  createEmailTemplate: vi.fn(),
  updateEmailTemplate: vi.fn(),
}))

describe('App', () => {
  beforeEach(() => {
    me.mockResolvedValue({
      user: { id: 'u1', email: 'ada@acme.com', name: 'Ada', status: 'active' },
      memberships: [],
      platform_admin: false,
    })
    listOrganisations.mockResolvedValue({
      items: [{ id: '1', name: 'Acme', slug: 'acme', status: 'active' }],
    })
    getOrganisation.mockResolvedValue({ id: '1', name: 'Acme', slug: 'acme', status: 'active' })
    listMemberships.mockResolvedValue({ items: [] })
    listRoles.mockResolvedValue({ items: [] })
    listPermissions.mockResolvedValue({ items: [] })
    authProviders.mockResolvedValue({ google: true, dev_login: false })
  })

  it('routes to org workspace and shows section links', async () => {
    render(
      <MemoryRouter initialEntries={['/orgs/1/members']}>
        <App />
      </MemoryRouter>,
    )
    expect(await screen.findByLabelText('Switch organisation')).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Acme' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Members' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Members' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Create member' })).toBeInTheDocument()
  })
})
