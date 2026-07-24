import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  flattenAutomationConditions,
  getAutomation,
  listAutomationRuns,
  type Automation,
  type AutomationRun,
} from '../../api'
import { PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'
import { AutomationDag } from './dag/AutomationDag'
import { normalizeGraph } from './dag/build_graph'

export function AutomationsShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<Automation | null>(null)
  const [runs, setRuns] = useState<AutomationRun[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      try {
        const [a, r] = await Promise.all([
          getAutomation(id),
          listAutomationRuns(orgId, id),
        ])
        setItem(a)
        setRuns(r.items)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Not found')
      }
    })()
  }, [orgId, id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  const flat = flattenAutomationConditions(item.conditions)
  const graph = normalizeGraph({
    trigger: item.trigger,
    conditions: flat.items,
    actions: item.actions ?? [],
  }) // read-only: no invented defaults

  return (
    <section>
      <PageHeader title={item.name || 'Automation'} />
      <p className="lede">
        {item.enabled ? 'Enabled' : 'Disabled'} · trigger <code>{item.trigger}</code>
      </p>
      <AutomationDag
        readOnly
        trigger={graph.trigger}
        conditionMode={flat.mode}
        conditions={graph.conditions}
        actions={graph.actions}
      />
      <h3>Recent runs</h3>
      {runs.length === 0 ? (
        <p className="muted">No runs yet. Create an app user that matches the conditions.</p>
      ) : (
        <ul className="run-list">
          {runs.map((r) => (
            <li key={r.id}>
              <strong>{r.status}</strong> — {r.detail || '—'}{' '}
              <span className="muted">{new Date(r.created_at).toLocaleString()}</span>
            </li>
          ))}
        </ul>
      )}
      <div className="form-actions">
        <Link className="ghost" to={resourcePath(orgId, 'automations')}>
          Back
        </Link>
        <Link className="button" to={resourcePath(orgId, 'automations', item.id, 'edit')}>
          Edit
        </Link>
      </div>
    </section>
  )
}
