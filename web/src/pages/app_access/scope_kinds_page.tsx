import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { createAppScopeType, deleteAppScopeType, listAppScopeTypes } from '../../api'
import { ResourceTable } from '../../crud/ui'
import { AccessHeader, useResource } from './shared'

export function ScopeKindsPage() {
  const { orgId = '' } = useParams()
  const { data, error, loading, run } = useResource(() => listAppScopeTypes(orgId), [orgId])
  const [kind, setKind] = useState('')

  return (
    <section>
      <AccessHeader title="Scope kinds">
        The levels your product has, such as <code>project</code> or <code>environment</code>. You
        declare the kind here; the ids stay in your system and are never registered with us, so a
        grant can name a project we have never heard of.
      </AccessHeader>

      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault()
          void run(async () => {
            await createAppScopeType(orgId, { kind })
            setKind('')
          })
        }}
      >
        <input
          value={kind}
          onChange={(e) => setKind(e.target.value)}
          placeholder="project"
          aria-label="Scope kind"
          required
        />
        <button type="submit">Add scope kind</button>
      </form>

      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Kind', 'Label']}
          empty="No scope kinds yet. Add one before granting."
          rows={(data?.items ?? []).map((s) => ({
            key: s.id,
            cells: [<code key="k">{s.kind}</code>, s.label || '—'],
            actions: (
              <button type="button" onClick={() => void run(() => deleteAppScopeType(orgId, s.id))}>
                Delete
              </button>
            ),
          }))}
        />
      )}
    </section>
  )
}
