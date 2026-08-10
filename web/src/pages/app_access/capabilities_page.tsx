import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { createAppCapability, deleteAppCapability, listAppCapabilities } from '../../api'
import { ResourceTable } from '../../crud/ui'
import { AccessHeader, useResource } from './shared'

export function CapabilitiesPage() {
  const { orgId = '' } = useParams()
  const { data, error, loading, run } = useResource(() => listAppCapabilities(orgId), [orgId])
  const [key, setKey] = useState('')
  const [description, setDescription] = useState('')

  return (
    <section>
      <AccessHeader title="Capabilities">
        The verbs your product checks, written as <code>resource:action</code>. A role can only use
        capabilities declared here, so a typo is caught now rather than silently granting nothing.
      </AccessHeader>

      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault()
          void run(async () => {
            await createAppCapability(orgId, { key, description })
            setKey('')
            setDescription('')
          })
        }}
      >
        <input
          value={key}
          onChange={(e) => setKey(e.target.value)}
          placeholder="deploy:production"
          aria-label="Capability key"
          required
        />
        <input
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="What it allows (optional)"
          aria-label="Description"
        />
        <button type="submit">Add capability</button>
      </form>

      {error && <p className="error">{error}</p>}
      {loading ? (
        <p>Loading…</p>
      ) : (
        <ResourceTable
          columns={['Key', 'Description']}
          empty="No capabilities yet. Declare one before building a role."
          rows={(data?.items ?? []).map((c) => ({
            key: c.id,
            cells: [<code key="k">{c.key}</code>, c.description || '—'],
            actions: (
              <button type="button" onClick={() => void run(() => deleteAppCapability(orgId, c.id))}>
                Delete
              </button>
            ),
          }))}
        />
      )}
    </section>
  )
}
