import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  getAppUser,
  getAppUserAccess,
  listGroupsForAppUser,
  type AppAccessSet,
  type AppUser,
} from '../../api'
import { ConceptDocsLink } from '../../components/ConceptDocsLink'
import { ReachablePanel } from './reachable_panel'
import { DetailList, PageHeader, ResourceTable } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

/**
 * The API reports provenance as machine tokens such as
 * "group:au_customers app-role:editor". Reading that on screen is the whole
 * point of the section, so turn it into a sentence.
 */
function describeSource(source: string): string {
  if (!source) return 'a direct grant'
  const parts = source.split(' ')
  const group = parts.find((p) => p.startsWith('group:'))?.slice('group:'.length)
  const role = parts.find((p) => p.startsWith('app-role:'))?.slice('app-role:'.length)
  // The capabilities column already says what the grant carries, so a wildcard
  // adds nothing here and would just repeat itself.
  const via = role ? `the ${role} role, ` : ''
  // The everyone rule is the one grant nobody issued for this person in
  // particular, so it has to name itself plainly.
  if (parts.includes('everyone')) return `${via}granted to every customer`
  if (group) return `${via}via the ${group} group`
  return `${via}granted directly`
}

type AccessGrant = AppAccessSet['grants'][number]

/**
 * A wildcard grant lists no capabilities, so rendering the list alone would
 * show "none" for the grant that carries the most. Exclusions matter for the
 * same reason: a row that reads wider than it is sends someone looking for a
 * grant that does not exist.
 */
function describeCapabilities(g: AccessGrant): string {
  const base = g.all_capabilities ? 'every capability' : g.capabilities.join(', ') || 'none'
  const parts = [base]
  if (g.except_capabilities.length) parts.push(`except ${g.except_capabilities.join(', ')}`)
  if (g.constraint === 'self_subject') parts.push('on their own rows only')
  return parts.join(', ')
}

function describeScope(g: AccessGrant): string {
  const base = `${g.scope_kind} / ${g.scope_id}`
  if (!g.except_scopes.length) return base
  return `${base}, except ${g.except_scopes.map((s) => `${s.kind}/${s.id}`).join(', ')}`
}

export function UsersShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<AppUser | null>(null)
  const [groups, setGroups] = useState<{ id: string; name: string }[]>([])
  const [access, setAccess] = useState<AppAccessSet | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAppUser(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
    // Access is shown here rather than on a lookup page: the question is always
    // asked about a person you are already looking at. A failure to read it must
    // not hide the user itself, so it is kept out of the error state.
    void listGroupsForAppUser(id)
      .then((r) => setGroups(r.items))
      .catch(() => setGroups([]))
    void getAppUserAccess(id)
      .then(setAccess)
      .catch(() => setAccess(null))
  }, [id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="User" />
      <DetailList
        items={[
          { label: 'Display name', value: item.display_name || '—' },
          { label: 'Email', value: item.email || '—' },
          { label: 'External ID', value: item.external_id || '—' },
          { label: 'Status', value: item.status },
          {
            label: 'Attributes',
            value:
              Object.keys(item.attributes || {}).length === 0
                ? '—'
                : Object.entries(item.attributes)
                    .map(([k, v]) => `${k}=${String(v)}`)
                    .join(', '),
          },
        ]}
      />
      <h3 className="section-title">Groups</h3>
      {groups.length === 0 ? (
        <p className="muted">In no groups.</p>
      ) : (
        <p>
          {groups.map((g, i) => (
            <span key={g.id}>
              {i > 0 && ', '}
              <Link to={resourcePath(orgId, 'customer-groups', g.id)}>{g.name}</Link>
            </span>
          ))}
        </p>
      )}

      <h3 className="section-title">Effective access</h3>
      <p className="muted">
        What this customer can do in your app, and where each capability comes from.{' '}
        <ConceptDocsLink slug="customer-access" label="How customer access works" />
      </p>
      <ResourceTable
        actions={false}
        columns={['Scope', 'Capabilities', 'Through', 'Expires']}
        empty="No access granted."
        rows={(access?.grants ?? []).map((g) => ({
          key: g.id,
          cells: [
            describeScope(g),
            describeCapabilities(g),
            describeSource(g.source),
            g.expires_at ? new Date(g.expires_at).toLocaleDateString() : 'never',
          ],
        }))}
      />

      <ReachablePanel appUserId={item.id} />

      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'users')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'users', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
