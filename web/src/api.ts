const API_BASE = import.meta.env.VITE_API_BASE ?? ''
const TOKEN_KEY = 'kyc_session_token'

export type Organisation = {
  id: string
  name: string
  slug: string
  status: string
}

export type User = {
  id: string
  email: string
  name: string
  status: string
  platform_admin?: boolean
}

export type Membership = {
  id: string
  organisation_id: string
  user_id: string
  role_id: string
  status: string
  user_email?: string
  user_name?: string
  role_key?: string
  organisation_name?: string
  organisation_slug?: string
}

export type Role = {
  id: string
  key: string
  name: string
  description?: string
  is_system?: boolean
  permission_keys?: string[]
}

export type Permission = {
  id: string
  key: string
  resource: string
  action: string
  category: string
  description: string
}

export type MeResponse = {
  user: User
  memberships: Membership[]
  platform_admin: boolean
}

export type AuthProviders = {
  google: boolean
  dev_login: boolean
}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

/** Capture `#token=` from Google OAuth redirect and persist it. */
export function captureOAuthTokenFromHash(): boolean {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return false
  const params = new URLSearchParams(hash)
  const token = params.get('token')
  if (!token) return false
  setToken(token)
  history.replaceState(null, '', window.location.pathname + window.location.search)
  return true
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string> | undefined),
  }
  const token = getToken()
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const message = data?.error?.message ?? res.statusText
    const err = new Error(message) as Error & { status?: number; code?: string }
    err.status = res.status
    err.code = data?.error?.code
    throw err
  }
  return data as T
}

export type AuthResponse = {
  token: string
  expires_at: string
  user: User
}

export function authProviders() {
  return request<AuthProviders>('/v1/auth/providers')
}

export function googleAuthURL() {
  return `${API_BASE}/v1/auth/google`
}

export function devLogin(email: string, name: string) {
  return request<AuthResponse>('/v1/auth/dev-login', {
    method: 'POST',
    body: JSON.stringify({ email, name }),
  })
}

export async function logout() {
  try {
    await request<{ ok: boolean }>('/v1/auth/logout', { method: 'POST' })
  } finally {
    setToken(null)
  }
}

export function me() {
  return request<MeResponse>('/v1/me')
}

export function listOrganisations() {
  return request<{ items: Organisation[] }>('/v1/organisations')
}

export function createOrganisation(name: string, slug?: string) {
  return request<Organisation>('/v1/organisations', {
    method: 'POST',
    body: JSON.stringify({ name, slug: slug || undefined }),
  })
}

export function getOrganisation(id: string) {
  return request<Organisation>(`/v1/organisations/${id}`)
}

export function listMemberships(orgId: string) {
  return request<{ items: Membership[] }>(`/v1/organisations/${orgId}/memberships`)
}

export function getMembership(id: string) {
  return request<Membership>(`/v1/memberships/${id}`)
}

export function inviteMember(orgId: string, email: string, roleId: string) {
  return request<Membership>(`/v1/organisations/${orgId}/memberships`, {
    method: 'POST',
    body: JSON.stringify({ email, role_id: roleId }),
  })
}

export function updateMembership(
  id: string,
  input: { role_id?: string; status?: string },
) {
  return request<Membership>(`/v1/memberships/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteMembership(id: string) {
  return request<Membership>(`/v1/memberships/${id}`, { method: 'DELETE' })
}

export function listRoles(orgId: string) {
  return request<{ items: Role[] }>(`/v1/organisations/${orgId}/roles`)
}

export function getRole(id: string) {
  return request<Role>(`/v1/roles/${id}`)
}

export function listPermissions() {
  return request<{ items: Permission[] }>('/v1/permissions')
}

export function updateRole(
  roleId: string,
  input: { name?: string; description?: string; permission_keys?: string[] },
) {
  return request<Role>(`/v1/roles/${roleId}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function createRole(
  orgId: string,
  input: { key: string; name: string; description?: string; permission_keys: string[] },
) {
  return request<Role>(`/v1/organisations/${orgId}/roles`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function deleteRole(id: string) {
  return request<{ ok: boolean }>(`/v1/roles/${id}`, { method: 'DELETE' })
}

export type Plan = {
  id: string
  key: string
  name: string
  status: string
  entitlement_keys: string[]
}

export type Entitlement = {
  id: string
  key: string
  description: string
}

export type Subscription = {
  id: string
  organisation_id: string
  plan_id: string
  status: string
}

export function listPlans() {
  return request<{ items: Plan[] }>('/v1/plans')
}

export function listEntitlementsCatalog() {
  return request<{ items: Entitlement[] }>('/v1/entitlements')
}

export function getSubscription(orgId: string) {
  return request<Subscription>(`/v1/organisations/${orgId}/subscription`)
}

export function getOrgEntitlements(orgId: string) {
  return request<{ entitlements: string[] }>(`/v1/organisations/${orgId}/entitlements`)
}

export type AttributeDefinition = {
  id: string
  organisation_id: string
  key: string
  label: string
  description: string
  value_type: 'string' | 'number' | 'boolean' | 'date' | 'dropdown'
  section: string
  sort_order: number
  required: boolean
  enum_values: string[]
  is_pii: boolean
  status: string
}

export type AppUser = {
  id: string
  organisation_id: string
  external_id: string | null
  email: string | null
  display_name: string
  status: string
  attributes: Record<string, unknown>
}

export function listAttributeDefinitions(orgId: string, status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : ''
  return request<{ items: AttributeDefinition[] }>(
    `/v1/organisations/${orgId}/attribute-definitions${q}`,
  )
}

export function getAttributeDefinition(id: string) {
  return request<AttributeDefinition>(`/v1/attribute-definitions/${id}`)
}

export function createAttributeDefinition(
  orgId: string,
  input: {
    key: string
    label: string
    description?: string
    value_type: string
    section?: string
    sort_order?: number
    required?: boolean
    enum_values?: string[]
    is_pii?: boolean
  },
) {
  return request<AttributeDefinition>(`/v1/organisations/${orgId}/attribute-definitions`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateAttributeDefinition(
  id: string,
  input: {
    label?: string
    description?: string
    value_type?: string
    section?: string
    sort_order?: number
    required?: boolean
    enum_values?: string[]
    is_pii?: boolean
    status?: string
  },
) {
  return request<AttributeDefinition>(`/v1/attribute-definitions/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteAttributeDefinition(id: string) {
  return request<AttributeDefinition>(`/v1/attribute-definitions/${id}`, { method: 'DELETE' })
}

export function listAppUsers(orgId: string, status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : ''
  return request<{ items: AppUser[] }>(`/v1/organisations/${orgId}/app-users${q}`)
}

export function getAppUser(id: string) {
  return request<AppUser>(`/v1/app-users/${id}`)
}

export function createAppUser(
  orgId: string,
  input: {
    email?: string
    external_id?: string
    display_name?: string
    status?: string
    attributes?: Record<string, unknown>
  },
) {
  return request<AppUser>(`/v1/organisations/${orgId}/app-users`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateAppUser(
  id: string,
  input: {
    email?: string
    external_id?: string
    display_name?: string
    status?: string
    attributes?: Record<string, unknown>
  },
) {
  return request<AppUser>(`/v1/app-users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteAppUser(id: string) {
  return request<AppUser>(`/v1/app-users/${id}`, { method: 'DELETE' })
}

export type EmailTemplate = {
  id: string
  organisation_id: string
  key: string
  name: string
  description: string
  subject: string
  body_text: string
  body_html: string
  status: string
  is_system: boolean
}

export function listEmailTemplates(orgId: string, status?: string) {
  const q = status ? `?status=${encodeURIComponent(status)}` : ''
  return request<{ items: EmailTemplate[] }>(
    `/v1/organisations/${orgId}/email-templates${q}`,
  )
}

export function createEmailTemplate(
  orgId: string,
  input: {
    key: string
    name: string
    description?: string
    subject: string
    body_text?: string
    body_html?: string
  },
) {
  return request<EmailTemplate>(`/v1/organisations/${orgId}/email-templates`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateEmailTemplate(
  id: string,
  input: {
    name?: string
    description?: string
    subject?: string
    body_text?: string
    body_html?: string
    status?: string
  },
) {
  return request<EmailTemplate>(`/v1/email-templates/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function getEmailTemplate(id: string) {
  return request<EmailTemplate>(`/v1/email-templates/${id}`)
}

export function deleteEmailTemplate(id: string) {
  return request<EmailTemplate>(`/v1/email-templates/${id}`, { method: 'DELETE' })
}
