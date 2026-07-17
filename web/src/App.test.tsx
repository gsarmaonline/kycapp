import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App from './App'

vi.mock('./api', () => ({
  listOrganisations: vi.fn(async () => ({
    items: [{ id: '1', name: 'Acme', slug: 'acme', status: 'active' }],
  })),
  createOrganisation: vi.fn(),
  getOrganisation: vi.fn(),
  listMemberships: vi.fn(),
  listRoles: vi.fn(),
  listPermissions: vi.fn(),
  inviteMember: vi.fn(),
  updateRole: vi.fn(),
  createRole: vi.fn(),
}))

describe('App', () => {
  it('renders organisations heading and loaded org', async () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Organisations' })).toBeInTheDocument()
    expect(await screen.findByText('Acme')).toBeInTheDocument()
  })
})
