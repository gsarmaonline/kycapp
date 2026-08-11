import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { deleteAppGrant, listAppGrants, type AppGrant } from '../../api'
import { ConceptDocsLink } from '../../components/ConceptDocsLink'
import { PageHeader, ResourceTable } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

function subjectLabel(kind: AppGrant['subject_kind']): string {
  if (kind === 'group') return 'Group'
  if (kind === 'everyone') return 'Everyone'
  return 'Customer'
}

function carries(g: AppGrant): string {
  const base = g.all_capabilities ? 'every capability' : g.role_key
  return g.constraint === 'self_subject' ? `${base}, own rows only` : base
}

function exceptSummary(g: AppGrant): string {
  const parts: string[] = []
  if (g.except_capabilities.length) parts.push(g.except_capabilities.join(', '))
  if (g.except_scopes.length) parts.push(g.except_scopes.map((s) => `${s.kind}/${s.id}`).join(', '))
  if (g.except_app_users.length) {
    parts.push(`${g.except_app_users.length} customer${g.except_app_users.length > 1 ? 's' : ''}`)
  }
  return parts.join('; ')
}

export function CustomerGrantsIndex() {
  const { orgId = '' } = useParams()
  const [items, setItems] = useState<AppGrant[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    setLoading(true)
    setError(null)
    try {
      setItems((await listAppGrants(orgId)).items)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load grants')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
  }, [orgId])

  async function onRevoke(g: AppGrant) {
    if (!confirm(`Revoke ${g.role_key} on ${g.scope_kind}/${g.scope_id} from ${g.subject_label}?`))
      return
    try {
      await deleteAppGrant(orgId, g.id)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Revoke failed')
    }
  }

  return (
    <section>
      <PageHeader
        title="Grants"
        createTo={resourcePath(orgId, 'customer-grants', 'new')}
        createLabel="Grant access"
      />
      <p className="muted">
        Each row gives one subject one set of capabilities over one scope. Grants are never edited in place: revoke
        and issue a new one, so the history stays readable. Exceptions narrow the grant they sit on
        and nothing else, so no grant ever cancels another.{' '}
        <ConceptDocsLink slug="customer-grants" label="How grants are evaluated" />
      </p>
      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Subject', 'Type', 'Carries', 'Scope', 'Except', 'Expires']}
          empty="No grants yet."
          rows={items.map((g) => ({
            key: g.id,
            cells: [
              g.subject_label,
              subjectLabel(g.subject_kind),
              carries(g),
              `${g.scope_kind} / ${g.scope_id}`,
              // Exclusions belong in the list, not behind a click. A grant that
              // reads as wider than it is invites someone to issue a second one.
              exceptSummary(g) || '—',
              g.expires_at ? new Date(g.expires_at).toLocaleDateString() : 'never',
            ],
            actions: (
              <button type="button" className="link-btn danger" onClick={() => void onRevoke(g)}>
                Revoke
              </button>
            ),
          }))}
        />
      )}
    </section>
  )
}
