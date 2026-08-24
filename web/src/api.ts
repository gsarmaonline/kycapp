import type { EmailBodySection, EmailTypography } from './email_fonts'

const API_BASE = import.meta.env.VITE_API_BASE ?? ''
const TOKEN_KEY = 'kyc_session_token'

export type Organisation = {
  id: string
  name: string
  slug: string
  status: string
  logo_url?: string
  primary_color?: string
  accent_color?: string
  email_footer?: string
  email_font?: string
  email_typography?: EmailTypography
  email_from_name?: string
  email_from_address?: string
  app_user_authority?: 'kyc' | 'external'
  app_user_ingest_upsert_key?: 'external_id' | 'email'
  app_user_attributes_mode?: 'strict' | 'discover'
}

export type User = {
  id: string
  email: string
  name: string
  status: string
  avatar_url?: string | null
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
  return request<{ items: Organisation[] }>('/v1/organisations?status=active')
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

export type OnboardingStep = {
  key: string
  label: string
  done: boolean
  href: string
}

export type OrgOnboarding = {
  visible: boolean
  dismissed: boolean
  completed_count: number
  total_count: number
  steps: OnboardingStep[]
}

export function getOrgOnboarding(orgId: string) {
  return request<OrgOnboarding>(`/v1/organisations/${orgId}/onboarding`)
}

export function dismissOrgOnboarding(orgId: string) {
  return request<OrgOnboarding>(`/v1/organisations/${orgId}/onboarding`, {
    method: 'PATCH',
    body: JSON.stringify({ dismissed: true }),
  })
}

export function updateOrganisation(
  id: string,
  input: {
    name?: string
    status?: string
    primary_color?: string
    accent_color?: string
    email_footer?: string
    email_font?: string
    email_typography?: EmailTypography
    email_from_name?: string
    email_from_address?: string
    app_user_authority?: 'kyc' | 'external'
    app_user_ingest_upsert_key?: 'external_id' | 'email'
    app_user_attributes_mode?: 'strict' | 'discover'
  },
) {
  return request<Organisation>(`/v1/organisations/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteOrganisation(id: string) {
  return request<{ ok: boolean; id: string }>(`/v1/organisations/${id}`, { method: 'DELETE' })
}

export type OrgIntegration = {
  provider: string
  status: string
  secret_hint?: string
  public_key_hint?: string
  has_secret: boolean
  has_public_key: boolean
}

export type OrgAPIKey = {
  id: string
  name: string
  key_prefix: string
  scopes: string[]
  created_at: string
  last_used_at?: string
  revoked: boolean
  revoked_at?: string
  token?: string
}

export function listOrgIntegrations(orgId: string) {
  return request<{ items: OrgIntegration[] }>(`/v1/organisations/${orgId}/integrations`)
}

export function upsertStripeIntegration(
  orgId: string,
  input: { secret_key?: string; publishable_key?: string },
) {
  return request<OrgIntegration>(`/v1/organisations/${orgId}/integrations/stripe`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deleteOrgIntegration(orgId: string, provider: string) {
  return request<{ ok: boolean }>(`/v1/organisations/${orgId}/integrations/${provider}`, {
    method: 'DELETE',
  })
}

export type OrgDatabase = {
  id: string
  organisation_id: string
  name: string
  driver: string
  host: string
  port: number
  database_name: string
  username: string
  password_hint?: string
  has_password: boolean
  ssl_mode: string
  status: 'connected' | 'unreachable' | 'disconnected' | string
  last_checked_at?: string | null
  last_error?: string
}

export function listOrgDatabases(orgId: string) {
  return request<{ items: OrgDatabase[] }>(`/v1/organisations/${orgId}/databases`)
}

export function getOrgDatabase(orgId: string, dbId: string) {
  return request<OrgDatabase>(`/v1/organisations/${orgId}/databases/${dbId}`)
}

export function createOrgDatabase(
  orgId: string,
  input: {
    name: string
    host: string
    port?: number
    database_name: string
    username: string
    password: string
    ssl_mode?: string
  },
) {
  return request<OrgDatabase>(`/v1/organisations/${orgId}/databases`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateOrgDatabase(
  orgId: string,
  dbId: string,
  input: {
    name?: string
    host?: string
    port?: number
    database_name?: string
    username?: string
    password?: string
    ssl_mode?: string
  },
) {
  return request<OrgDatabase>(`/v1/organisations/${orgId}/databases/${dbId}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function checkOrgDatabase(orgId: string, dbId: string) {
  return request<OrgDatabase>(`/v1/organisations/${orgId}/databases/${dbId}/check`, {
    method: 'POST',
  })
}

export function disconnectOrgDatabase(orgId: string, dbId: string) {
  return request<OrgDatabase>(`/v1/organisations/${orgId}/databases/${dbId}/disconnect`, {
    method: 'POST',
  })
}

export function deleteOrgDatabase(orgId: string, dbId: string) {
  return request<{ ok: boolean }>(`/v1/organisations/${orgId}/databases/${dbId}`, {
    method: 'DELETE',
  })
}

export type OrgWebhook = {
  id: string
  organisation_id: string
  name: string
  url: string
  secret_hint?: string
  has_secret: boolean
  status: string
  body_template?: string
}

export function listOrgWebhooks(orgId: string) {
  return request<{ items: OrgWebhook[] }>(`/v1/organisations/${orgId}/webhooks`)
}

export function getOrgWebhook(orgId: string, webhookId: string) {
  return request<OrgWebhook>(`/v1/organisations/${orgId}/webhooks/${webhookId}`)
}

export function createOrgWebhook(
  orgId: string,
  input: { name: string; url: string; secret?: string; body_template?: string },
) {
  return request<OrgWebhook>(`/v1/organisations/${orgId}/webhooks`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateOrgWebhook(
  orgId: string,
  webhookId: string,
  input: {
    name?: string
    url?: string
    secret?: string
    status?: string
    body_template?: string
  },
) {
  return request<OrgWebhook>(`/v1/organisations/${orgId}/webhooks/${webhookId}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteOrgWebhook(orgId: string, webhookId: string) {
  return request<{ ok: boolean }>(`/v1/organisations/${orgId}/webhooks/${webhookId}`, {
    method: 'DELETE',
  })
}

export type InboundWebhook = {
  id: string
  organisation_id: string
  name: string
  url: string
  auth_mode: 'header' | 'query' | 'path' | string
  secret_hint?: string
  has_secret: boolean
  status: string
  /** Present after create/rotate, or always for query/path (embedded in URL) */
  secret?: string
}

export function listInboundWebhooks(orgId: string) {
  return request<{ items: InboundWebhook[] }>(`/v1/organisations/${orgId}/inbound-webhooks`)
}

export function getInboundWebhook(orgId: string, hookId: string) {
  return request<InboundWebhook>(`/v1/organisations/${orgId}/inbound-webhooks/${hookId}`)
}

export function createInboundWebhook(
  orgId: string,
  input: { name: string; secret?: string; status?: string; auth_mode?: string },
) {
  return request<InboundWebhook>(`/v1/organisations/${orgId}/inbound-webhooks`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateInboundWebhook(
  orgId: string,
  hookId: string,
  input: { name?: string; secret?: string; status?: string; auth_mode?: string; rotate?: boolean },
) {
  return request<InboundWebhook>(`/v1/organisations/${orgId}/inbound-webhooks/${hookId}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteInboundWebhook(orgId: string, hookId: string) {
  return request<{ ok: boolean }>(`/v1/organisations/${orgId}/inbound-webhooks/${hookId}`, {
    method: 'DELETE',
  })
}

export function listOrgAPIKeys(orgId: string) {
  return request<{ items: OrgAPIKey[] }>(`/v1/organisations/${orgId}/api-keys`)
}

export function createOrgAPIKey(orgId: string, input: { name: string; scopes?: string[] }) {
  return request<OrgAPIKey>(`/v1/organisations/${orgId}/api-keys`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function revokeAPIKey(id: string) {
  return request<{ id: string; revoked: boolean }>(`/v1/api-keys/${id}`, { method: 'DELETE' })
}

export async function uploadOrganisationLogo(id: string, file: File) {
  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  const body = new FormData()
  body.append('logo', file)
  const res = await fetch(`${API_BASE}/v1/organisations/${id}/branding/logo`, {
    method: 'POST',
    headers,
    body,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data?.error?.message ?? res.statusText)
  }
  return data as Organisation
}

export function deleteOrganisationLogo(id: string) {
  return request<Organisation>(`/v1/organisations/${id}/branding/logo`, {
    method: 'DELETE',
  })
}

export function listMemberships(orgId: string) {
  return request<{ items: Membership[] }>(`/v1/organisations/${orgId}/memberships`)
}

export function getMembership(id: string) {
  return request<Membership>(`/v1/memberships/${id}`)
}

/**
 * One edge the authorisation walk crossed.
 *
 * `redacted` means a name in this hop sits outside the organisation you are
 * looking at, so it is withheld rather than dropped: the route stays honest
 * about its length without naming what you cannot already see.
 */
export type PathHop = {
  object: string
  relation: string
  subject: string
  redacted: boolean
}

/**
 * `reason` is the engine's own vocabulary rather than a boolean, because the
 * distinction is the useful part. `unreachable` means no path arrives at all,
 * `no_rule` means one arrives but grants something else, and `excluded` means a
 * rule matched and a subtraction removed it. Each calls for a different fix.
 */
export type PermissionOutcome = {
  key: string
  allowed: boolean
  reason: 'allowed' | 'unreachable' | 'no_rule' | 'excluded'
  path: PathHop[]
}

export type AccessExplanation = {
  organisation_id: string
  subject: string
  outcomes: PermissionOutcome[]
}

export function explainMembershipAccess(id: string) {
  return request<AccessExplanation>(`/v1/memberships/${id}/access`)
}

/**
 * One of the organisation's own roles.
 *
 * `extends` is why owner, admin and member are not in the schema map: they are
 * rows, not types. The map draws what the model permits; these are instances of
 * it, and the inheritance between them is real but was visible nowhere.
 */
export type OperatorRole = {
  id: string
  key: string
  name: string
  extends: string[]
  permissions: string[]
}

export function listOperatorRoles(orgId: string) {
  return request<{ items: OperatorRole[] }>(`/v1/organisations/${orgId}/operator-roles`)
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

export type EntitlementScope = 'platform' | 'product'

export type Plan = {
  id: string
  key: string
  name: string
  status: string
  entitlement_keys: string[]
  platform_capability_keys?: string[]
  product_feature_keys?: string[]
}

export type Entitlement = {
  id: string
  key: string
  description: string
  scope: EntitlementScope
}

export type OrgEntitlements = {
  entitlements: string[]
  platform_capabilities: string[]
  product_features: string[]
}

export type Subscription = {
  id: string
  organisation_id: string
  plan_id: string
  status: string
  current_period_end?: string | null
  processor?: string
  subscription_ref?: string
}

export type PlanPrice = {
  id: string
  plan_id: string
  interval: string
  currency: string
  unit_amount: number
  processor: string
  processor_price_ref: string
  status: string
}

export function listPlans() {
  return request<{ items: Plan[] }>('/v1/plans')
}

export function listEntitlementsCatalog() {
  return request<{ items: Entitlement[] }>('/v1/entitlements')
}

export function listPlanPrices(planId: string) {
  return request<{ items: PlanPrice[] }>(`/v1/plans/${planId}/prices`)
}

export function getSubscription(orgId: string) {
  return request<Subscription>(`/v1/organisations/${orgId}/subscription`)
}

export function getOrgEntitlements(orgId: string) {
  return request<OrgEntitlements>(`/v1/organisations/${orgId}/entitlements`)
}

export type ProductFeatureOverride = {
  subject_id: string
  effect: 'include' | 'exclude'
}

export type ProductFeature = {
  id: string
  organisation_id?: string
  key: string
  description: string
  scope: 'product'
  enabled: boolean
  rollout_percentage: number
  overrides: ProductFeatureOverride[]
}

export type ProductPlanPrice = {
  id: string
  product_plan_id: string
  interval: string
  currency: string
  unit_amount: number
  processor: string
  processor_product_ref: string
  processor_price_ref: string
  status: string
  synced: boolean
}

export type ProductPlan = {
  id: string
  organisation_id: string
  key: string
  name: string
  status: string
  feature_keys: string[]
  prices?: ProductPlanPrice[]
  created_at?: string
  updated_at?: string
}

export type StripeCatalogItem = {
  product_ref: string
  product_name: string
  price_ref: string
  interval: string
  currency: string
  unit_amount: number
  active: boolean
}

export type StripeCatalogSyncResult = {
  imported: ProductPlan[]
  pushed: ProductPlan[]
  skipped: number
}

export function listProductFeatures(orgId: string) {
  return request<{ items: ProductFeature[] }>(`/v1/organisations/${orgId}/product-features`)
}

export function createProductFeature(
  orgId: string,
  input: { key: string; description?: string; enabled?: boolean; rollout_percentage?: number },
) {
  return request<ProductFeature>(`/v1/organisations/${orgId}/product-features`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getProductFeature(id: string) {
  return request<ProductFeature>(`/v1/product-features/${id}`)
}

export function updateProductFeature(
  id: string,
  input: { description?: string; enabled?: boolean; rollout_percentage?: number },
) {
  return request<ProductFeature>(`/v1/product-features/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function setProductFeatureOverrides(
  id: string,
  input: { overrides: ProductFeatureOverride[] },
) {
  return request<ProductFeature>(`/v1/product-features/${id}/overrides`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deleteProductFeature(id: string) {
  return request<{ ok: boolean }>(`/v1/product-features/${id}`, { method: 'DELETE' })
}

export function listProductPlans(orgId: string) {
  return request<{ items: ProductPlan[] }>(`/v1/organisations/${orgId}/product-plans`)
}

export function createProductPlan(
  orgId: string,
  input: {
    key: string
    name: string
    price?: { interval: string; currency?: string; unit_amount: number; status?: string }
  },
) {
  return request<ProductPlan>(`/v1/organisations/${orgId}/product-plans`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getProductPlan(id: string) {
  return request<ProductPlan>(`/v1/product-plans/${id}`)
}

export function updateProductPlan(id: string, input: { name?: string; status?: string }) {
  return request<ProductPlan>(`/v1/product-plans/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function setProductPlanFeatures(id: string, feature_keys: string[]) {
  return request<ProductPlan>(`/v1/product-plans/${id}/features`, {
    method: 'PUT',
    body: JSON.stringify({ feature_keys }),
  })
}

export function upsertProductPlanPrice(
  id: string,
  input: { interval: string; currency?: string; unit_amount: number; status?: string },
) {
  return request<ProductPlanPrice>(`/v1/product-plans/${id}/price`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function listProductPlanPrices(id: string) {
  return request<{ items: ProductPlanPrice[] }>(`/v1/product-plans/${id}/prices`)
}

export function deleteProductPlan(id: string) {
  return request<{ ok: boolean }>(`/v1/product-plans/${id}`, { method: 'DELETE' })
}

export function getActiveProductPlan(orgId: string) {
  return request<ProductPlan>(`/v1/organisations/${orgId}/product-plan`)
}

export function setActiveProductPlan(orgId: string, product_plan_id: string) {
  return request<{ product_plan: ProductPlan | null }>(`/v1/organisations/${orgId}/product-plan`, {
    method: 'PUT',
    body: JSON.stringify({ product_plan_id }),
  })
}

export function listStripeCatalog(orgId: string) {
  return request<{ items: StripeCatalogItem[] }>(
    `/v1/organisations/${orgId}/integrations/stripe/catalog`,
  )
}

export function importStripeCatalog(
  orgId: string,
  items: { price_ref: string; key?: string; name?: string }[],
) {
  return request<StripeCatalogSyncResult>(`/v1/organisations/${orgId}/integrations/stripe/import`, {
    method: 'POST',
    body: JSON.stringify({ items }),
  })
}

export function syncProductPlansToStripe(orgId: string) {
  return request<StripeCatalogSyncResult>(`/v1/organisations/${orgId}/integrations/stripe/sync`, {
    method: 'POST',
  })
}

export function createBillingCheckout(
  orgId: string,
  input: { plan_id: string; interval?: string; success_url?: string; cancel_url?: string },
) {
  return request<{ url: string }>(`/v1/organisations/${orgId}/billing/checkout`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function createBillingPortal(orgId: string, input?: { return_url?: string }) {
  return request<{ url: string }>(`/v1/organisations/${orgId}/billing/portal`, {
    method: 'POST',
    body: JSON.stringify(input ?? {}),
  })
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
  is_system: boolean
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
  body_sections?: EmailBodySection[]
  from_name?: string
  from_address?: string
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
    body_sections?: EmailBodySection[]
    from_name?: string
    from_address?: string
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
    body_sections?: EmailBodySection[]
    from_name?: string
    from_address?: string
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

export type AutomationConditionOp =
  | 'eq'
  | 'neq'
  | 'exists'
  | 'not_exists'
  | 'in'
  | 'not_in'
  | 'gt'
  | 'gte'
  | 'lt'
  | 'lte'
  | 'contains'

export type AutomationCondition = {
  field: string
  op: AutomationConditionOp | string
  value?: string | number | boolean | string[]
}

export type AutomationConditionMode = 'all' | 'any'

export type AutomationConditions = {
  mode?: AutomationConditionMode
  items?: AutomationCondition[]
  all?: AutomationCondition[]
  any?: AutomationCondition[]
}

export function flattenAutomationConditions(c?: AutomationConditions | null): {
  mode: AutomationConditionMode
  items: AutomationCondition[]
} {
  if (!c) return { mode: 'all', items: [] }
  if (c.items?.length) {
    return { mode: c.mode === 'any' ? 'any' : 'all', items: [...c.items] }
  }
  if ((c.any?.length ?? 0) > 0 && !(c.all?.length ?? 0)) {
    return { mode: 'any', items: [...(c.any ?? [])] }
  }
  return { mode: c.mode === 'any' ? 'any' : 'all', items: [...(c.all ?? [])] }
}

export type AutomationAction = {
  id?: string
  type: string
  params?: Record<string, string>
  /** Next action id when this step succeeds */
  on_success?: string
  /** Next action id when this step fails (omit = fail the run) */
  on_error?: string
  /** @deprecated legacy flat field; prefer params.template_key */
  template_key?: string
}

export type AutomationCatalog = {
  triggers: {
    id: string
    label: string
    description: string
    resource?: string
    kind?: string
    provides?: string[]
    params?: {
      key: string
      label: string
      required: boolean
      input: string
      options_from?: string
      enum_values?: string[]
      hint?: string
    }[]
  }[]
  actions: {
    type: string
    label: string
    description: string
    params: { key: string; label: string; required: boolean }[]
    requires?: string[]
  }[]
  ops: {
    op: string
    label: string
    needs_value: boolean
    needs_list?: boolean
    value_types?: string[]
  }[]
  condition_fields: {
    field: string
    label: string
    value_type: string
    group: string
    enum_values?: string[]
    allowed_ops?: string[]
  }[]
  databases?: { id: string; name: string }[]
  webhooks?: { id: string; name: string }[]
  inbound_webhooks?: { id: string; name: string }[]
  plans?: { id: string; name: string; key?: string }[]
  roles?: { id: string; name: string; key?: string }[]
  schedule_presets?: { key: string; label: string; expr: string }[]
}


export type Automation = {
  id: string
  organisation_id: string
  name: string
  trigger: string
  trigger_params?: Record<string, string>
  enabled: boolean
  conditions: AutomationConditions
  actions: AutomationAction[]
}

export type AutomationRun = {
  id: string
  organisation_id: string
  automation_id: string
  trigger: string
  status: string
  detail: string
  created_at: string
}

export function listAutomations(orgId: string) {
  return request<{ items: Automation[] }>(`/v1/organisations/${orgId}/automations`)
}

export function getAutomationCatalog(orgId: string) {
  return request<AutomationCatalog>(`/v1/organisations/${orgId}/automations/catalog`)
}

export function getAutomation(id: string) {
  return request<Automation>(`/v1/automations/${id}`)
}

export function createAutomation(
  orgId: string,
  input: {
    name: string
    trigger: string
    trigger_params?: Record<string, string>
    enabled?: boolean
    conditions: AutomationConditions
    actions: AutomationAction[]
  },
) {
  return request<Automation>(`/v1/organisations/${orgId}/automations`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateAutomation(
  id: string,
  input: {
    name?: string
    trigger?: string
    trigger_params?: Record<string, string>
    enabled?: boolean
    conditions?: AutomationConditions
    actions?: AutomationAction[]
  },
) {
  return request<Automation>(`/v1/automations/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteAutomation(id: string) {
  return request<{ ok: boolean }>(`/v1/automations/${id}`, { method: 'DELETE' })
}

export function listAutomationRuns(orgId: string, automationId?: string) {
  const q = automationId ? `?automation_id=${encodeURIComponent(automationId)}` : ''
  return request<{ items: AutomationRun[] }>(
    `/v1/organisations/${orgId}/automation-runs${q}`,
  )
}

export type ActivityEvent = {
  id: string
  organisation_id: string
  organisation_slug?: string
  organisation_name?: string
  actor_type?: string
  actor_id?: string
  actor_label?: string
  action: string
  resource_type?: string
  resource_id?: string
  summary: string
  payload?: Record<string, unknown>
  created_at: string
}

export type UsageCounter = {
  organisation_id: string
  meter_key: string
  period_start: string
  dim1_key: string
  dim1_value: string
  dim2_key: string
  dim2_value: string
  count: number
  updated_at: string
}

export function listOrgActivity(orgId: string, limit = 50) {
  const q = limit ? `?limit=${encodeURIComponent(String(limit))}` : ''
  return request<{ items: ActivityEvent[] }>(`/v1/organisations/${orgId}/activity${q}`)
}

export function listOrgUsage(orgId: string, opts?: { from?: string; to?: string }) {
  const params = new URLSearchParams()
  if (opts?.from) params.set('from', opts.from)
  if (opts?.to) params.set('to', opts.to)
  const q = params.toString() ? `?${params}` : ''
  return request<{ items: UsageCounter[] }>(`/v1/organisations/${orgId}/usage${q}`)
}

// --- Merchant-hosted access control ---
//
// The merchant's own model for their app users: scope kinds, capabilities,
// roles with inheritance, groups, and grants. Distinct from KYC's own roles and
// permissions, which govern the operators configuring all this.

export type AppScopeType = { id: string; kind: string; label: string }
export type AppCapability = { id: string; key: string; description: string }

export type AppRole = {
  id: string
  key: string
  name: string
  description: string
  own_capabilities: string[]
  /** What the role resolves to once inheritance is applied. */
  effective_capabilities: string[]
}

export type AppUserGroup = {
  id: string
  key: string
  name: string
  description: string
  member_count: number
  /**
   * Groups this one extends. A member of this group counts as a member of every
   * one of them, which is the same relation roles have always had. Absent on
   * responses that predate nesting.
   */
  parents?: string[]
  parent_count?: number
}

/** One excluded scope. Kind and id stay paired, never two parallel lists. */
export type AppScopeRef = { kind: string; id: string }

export type AppGrant = {
  id: string
  role_key: string
  scope_kind: string
  scope_id: string
  subject_kind: 'app_user' | 'group' | 'everyone'
  subject_label: string
  expires_at?: string
  /** Carries every capability in the org's namespace, including later ones. */
  all_capabilities: boolean
  except_capabilities: string[]
  except_scopes: AppScopeRef[]
  except_app_users: string[]
  constraint: '' | 'self_subject'
}

export type AppGroupMember = {
  id: string
  email: string | null
  display_name: string
  status: string
}

export function listAppScopeTypes(orgId: string) {
  return request<{ items: AppScopeType[] }>(`/v1/organisations/${orgId}/app-scope-types`)
}

export function createAppScopeType(orgId: string, body: { kind: string; label?: string }) {
  return request<AppScopeType>(`/v1/organisations/${orgId}/app-scope-types`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function deleteAppScopeType(orgId: string, id: string) {
  return request<void>(`/v1/organisations/${orgId}/app-scope-types/${id}`, { method: 'DELETE' })
}

export function listAppCapabilities(orgId: string) {
  return request<{ items: AppCapability[] }>(`/v1/organisations/${orgId}/app-capabilities`)
}

export function createAppCapability(orgId: string, body: { key: string; description?: string }) {
  return request<AppCapability>(`/v1/organisations/${orgId}/app-capabilities`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function deleteAppCapability(orgId: string, id: string) {
  return request<void>(`/v1/organisations/${orgId}/app-capabilities/${id}`, { method: 'DELETE' })
}

export function listAppRoles(orgId: string) {
  return request<{ items: AppRole[] }>(`/v1/organisations/${orgId}/app-roles`)
}

export function createAppRole(
  orgId: string,
  body: { key: string; name?: string; description?: string; capabilities?: string[]; extends?: string[] },
) {
  return request<AppRole>(`/v1/organisations/${orgId}/app-roles`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateAppRole(
  orgId: string,
  roleId: string,
  body: { name?: string; description?: string; capabilities?: string[]; extends?: string[] },
) {
  return request<AppRole>(`/v1/organisations/${orgId}/app-roles/${roleId}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteAppRole(orgId: string, roleId: string) {
  return request<void>(`/v1/organisations/${orgId}/app-roles/${roleId}`, { method: 'DELETE' })
}

export function listAppUserGroups(orgId: string) {
  return request<{ items: AppUserGroup[] }>(`/v1/organisations/${orgId}/app-user-groups`)
}

export function createAppUserGroup(
  orgId: string,
  body: { key: string; name?: string; description?: string; parents?: string[] },
) {
  return request<AppUserGroup>(`/v1/organisations/${orgId}/app-user-groups`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function deleteAppUserGroup(orgId: string, groupId: string) {
  return request<void>(`/v1/organisations/${orgId}/app-user-groups/${groupId}`, { method: 'DELETE' })
}

export function listAppGroupMembers(orgId: string, groupId: string) {
  return request<{ items: AppGroupMember[] }>(
    `/v1/organisations/${orgId}/app-user-groups/${groupId}/members`,
  )
}

export function addAppGroupMember(orgId: string, groupId: string, appUserId: string) {
  return request<{ ok: boolean }>(`/v1/organisations/${orgId}/app-user-groups/${groupId}/members`, {
    method: 'POST',
    body: JSON.stringify({ app_user_id: appUserId }),
  })
}

export function removeAppGroupMember(orgId: string, groupId: string, appUserId: string) {
  return request<void>(
    `/v1/organisations/${orgId}/app-user-groups/${groupId}/members/${appUserId}`,
    { method: 'DELETE' },
  )
}

export function listAppGrants(orgId: string) {
  return request<{ items: AppGrant[] }>(`/v1/organisations/${orgId}/app-grants`)
}

export function createAppGrant(
  orgId: string,
  body: {
    subject_kind: 'app_user' | 'group' | 'everyone'
    app_user_id?: string
    group_id?: string
    /** Omitted when all_capabilities is set: a wildcard grant carries no role. */
    role_id?: string
    scope_kind: string
    scope_id: string
    expires_at?: string
    all_capabilities?: boolean
    except_capabilities?: string[]
    except_scopes?: AppScopeRef[]
    except_app_user_ids?: string[]
    constraint?: '' | 'self_subject'
  },
) {
  return request<{ id: string }>(`/v1/organisations/${orgId}/app-grants`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function deleteAppGrant(orgId: string, grantId: string) {
  return request<void>(`/v1/organisations/${orgId}/app-grants/${grantId}`, { method: 'DELETE' })
}

export function updateAppUserGroup(
  orgId: string,
  groupId: string,
  body: { name?: string; description?: string; parents?: string[] },
) {
  return request<AppUserGroup>(`/v1/organisations/${orgId}/app-user-groups/${groupId}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

/** Which groups a customer belongs to. Half of "why do they have this access?". */
export function listGroupsForAppUser(appUserId: string) {
  return request<{ items: { id: string; key: string; name: string }[] }>(
    `/v1/app-users/${appUserId}/groups`,
  )
}

export type AppAccessSet = {
  app_user_id: string
  namespace: string
  version: number
  grants: {
    id: string
    scope_kind: string
    scope_id: string
    capabilities: string[]
    /** Names the group or rule a capability came through, when it did. */
    source: string
    expires_at?: string
    all_capabilities: boolean
    except_capabilities: string[]
    except_scopes: AppScopeRef[]
    constraint: '' | 'self_subject'
  }[]
}

export function getAppUserAccess(appUserId: string) {
  return request<AppAccessSet>(`/v1/app-users/${appUserId}/access`)
}

export function getAppScopeType(orgId: string, id: string) {
  return request<AppScopeType>(`/v1/organisations/${orgId}/app-scope-types/${id}`)
}

export function updateAppScopeType(orgId: string, id: string, body: { label: string }) {
  return request<AppScopeType>(`/v1/organisations/${orgId}/app-scope-types/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function getAppCapability(orgId: string, id: string) {
  return request<AppCapability>(`/v1/organisations/${orgId}/app-capabilities/${id}`)
}

export function updateAppCapability(orgId: string, id: string, body: { description: string }) {
  return request<AppCapability>(`/v1/organisations/${orgId}/app-capabilities/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

/** A role with the ids of the roles it builds on, which the edit form needs. */
export function getAppRole(orgId: string, roleId: string) {
  return request<AppRole & { extends: string[] }>(
    `/v1/organisations/${orgId}/app-roles/${roleId}`,
  )
}

export function getAppUserGroup(orgId: string, groupId: string) {
  return request<Omit<AppUserGroup, 'member_count'>>(
    `/v1/organisations/${orgId}/app-user-groups/${groupId}`,
  )
}
