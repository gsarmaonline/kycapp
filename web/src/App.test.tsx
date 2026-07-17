import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import App from './App'

const me = vi.fn()
const listOrganisations = vi.fn()
const authProviders = vi.fn()

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
  getOrganisation: vi.fn(),
  listMemberships: vi.fn(),
  listRoles: vi.fn(),
  listPermissions: vi.fn(),
  inviteMember: vi.fn(),
  updateRole: vi.fn(),
  createRole: vi.fn(),
  listPlans: vi.fn(),
  listEntitlementsCatalog: vi.fn(),
  getSubscription: vi.fn(),
  getOrgEntitlements: vi.fn(),
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
    authProviders.mockResolvedValue({ google: true, dev_login: false })
  })

  it('renders organisations after session load', async () => {
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Your organisations' })).toBeInTheDocument()
    expect(await screen.findByText('Acme')).toBeInTheDocument()
  })
})
