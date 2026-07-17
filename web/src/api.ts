const API_BASE = import.meta.env.VITE_API_BASE ?? ''

export type Organisation = {
  id: string
  name: string
  slug: string
  status: string
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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const message = data?.error?.message ?? res.statusText
    throw new Error(message)
  }
  return data as T
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

export function listRoles(orgId: string) {
  return request<{ items: Role[] }>(`/v1/organisations/${orgId}/roles`)
}

export function inviteMember(orgId: string, email: string, roleId: string) {
  return request<Membership>(`/v1/organisations/${orgId}/memberships`, {
    method: 'POST',
    body: JSON.stringify({ email, role_id: roleId }),
  })
}

export function listPermissions() {
  return request<{ items: Permission[] }>('/v1/permissions')
}

export function updateRole(roleId: string, permissionKeys: string[]) {
  return request<Role>(`/v1/roles/${roleId}`, {
    method: 'PATCH',
    body: JSON.stringify({ permission_keys: permissionKeys }),
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
