import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import App from './App'
import { getToken } from './api'

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
  getOrgEntitlements: vi.fn(async () => ({
    entitlements: [],
    platform_capabilities: [],
    product_features: [],
  })),
  listAttributeDefinitions: vi.fn(async () => ({ items: [] })),
  createAttributeDefinition: vi.fn(),
  listAppUsers: vi.fn(async () => ({ items: [] })),
  createAppUser: vi.fn(),
  listEmailTemplates: vi.fn(async () => ({ items: [] })),
  createEmailTemplate: vi.fn(),
  updateEmailTemplate: vi.fn(),
  listAutomations: vi.fn(async () => ({ items: [] })),
  listProductFeatures: vi.fn(async () => ({ items: [] })),
  listProductPlans: vi.fn(async () => ({ items: [] })),
  getOrgOnboarding: vi.fn(async () => ({
    visible: false,
    dismissed: false,
    completed_count: 0,
    total_count: 6,
    steps: [],
  })),
  dismissOrgOnboarding: vi.fn(),
  getActiveProductPlan: vi.fn(async () => {
    throw new Error('not found')
  }),
}))

vi.mock('swagger-ui-react', () => ({
  default: () => <div data-testid="swagger-ui-mock">OpenAPI</div>,
}))

vi.mock('swagger-ui-react/swagger-ui.css', () => ({}))

describe('App', () => {
  beforeEach(() => {
    vi.mocked(getToken).mockReturnValue('test-token')
    me.mockResolvedValue({
      user: { id: 'u1', email: 'ada@acme.com', name: 'Ada', status: 'active' },
      memberships: [],
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

  it('shows Sign in on the landing page when signed out', async () => {
    vi.mocked(getToken).mockReturnValue(null)
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )
    expect(
      await screen.findByRole('heading', { name: /system of record for organisations/i }),
    ).toBeInTheDocument()
    const signIns = screen.getAllByRole('link', { name: 'Sign in' })
    expect(signIns.some((el) => el.getAttribute('href') === '/login')).toBe(true)
    const docs = screen.getAllByRole('link', { name: 'Docs' })
    expect(docs.some((el) => el.getAttribute('href') === '/docs')).toBe(true)
  })

  it('shows public docs without requiring a session', async () => {
    vi.mocked(getToken).mockReturnValue(null)
    render(
      <MemoryRouter initialEntries={['/docs']}>
        <App />
      </MemoryRouter>,
    )
    expect(await screen.findByRole('heading', { name: 'Documentation' })).toBeInTheDocument()
    const nav = screen.getByRole('navigation', { name: 'Documentation sections' })
    expect(nav.querySelector('a[href="/docs"]')).toBeTruthy()
    expect(nav.querySelector('a[href="/docs/api"]')).toBeTruthy()
    expect(nav.querySelector('a[href="/docs/variables"]')).toBeTruthy()
    // Both API references are reachable from anywhere in the docs now, not only
    // once you are already inside the API section.
    expect(nav.querySelector('a[href="/docs/api/operator"]')).toBeTruthy()
    // Individual concepts are in the sidebar rather than only on the index, so
    // reading one no longer costs a trip through a list.
    expect(nav.querySelector('a[href="/docs/concepts/organisation"]')).toBeTruthy()
    // The first group orients rather than categorising: members against app
    // users, and platform against app, run through every section and used to be
    // explained inside customer access.
    expect(await screen.findByRole('heading', { name: 'Getting started' })).toBeInTheDocument()
    expect(nav.querySelector('a[href="/docs/concepts/getting-started"]')).toBeTruthy()
    expect(
      document.querySelector('a[href="/docs/concepts/organisation"]'),
    ).toBeTruthy()
    expect(screen.queryByLabelText('Organisation navigation')).not.toBeInTheDocument()
  })

  it('shows API reference nested under docs', async () => {
    vi.mocked(getToken).mockReturnValue(null)
    render(
      <MemoryRouter initialEntries={['/docs/api']}>
        <App />
      </MemoryRouter>,
    )
    // The two references used to appear in a second tab row that only rendered
    // once you were inside the API section. They live in the one sidebar now.
    const nav = await screen.findByRole('navigation', { name: 'Documentation sections' })
    expect(nav.querySelector('a[href="/docs/api"]')).toBeTruthy()
    expect(nav.querySelector('a[href="/docs/api/operator"]')).toBeTruthy()
  })

  it('marks the current docs page in the sidebar', async () => {
    vi.mocked(getToken).mockReturnValue(null)
    render(
      <MemoryRouter initialEntries={['/docs/variables']}>
        <App />
      </MemoryRouter>,
    )
    const nav = await screen.findByRole('navigation', { name: 'Documentation sections' })
    const current = nav.querySelector('a[href="/docs/variables"]')
    expect(current?.className).toContain('active')
    // Showing the whole tree is only useful if the current page is findable in
    // it, which is the thing a tab row was hiding.
    expect(nav.querySelector('a[href="/docs/api"]')?.className).not.toContain('active')
  })

  it('shows Dashboard on the landing page when signed in', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )
    const dashboards = await screen.findAllByRole('link', { name: /^Dashboard$/ })
    expect(dashboards.length).toBeGreaterThan(0)
    expect(dashboards.every((el) => el.getAttribute('href') === '/app')).toBe(true)
    // Session may still be resolving on first paint; assert final CTA targets /app.
    expect(dashboards.some((el) => el.classList.contains('landing-cta-primary'))).toBe(true)
  })
})
