import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  flattenAutomationConditions,
  getAutomation,
  getAutomationCatalog,
  listAutomationRuns,
  type Automation,
  type AutomationCatalog,
  type AutomationRun,
} from '../../api'
import { PageHeader, ShowActions } from '../../crud/ui'
import { resourcePath } from '../../org_nav'
import { AutomationDag } from './dag/AutomationDag'
import { normalizeGraph } from './dag/build_graph'

export function AutomationsShow() {
  const { orgId = '', id = '' } = useParams()
  const [item, setItem] = useState<Automation | null>(null)
  const [catalog, setCatalog] = useState<AutomationCatalog | null>(null)
  const [runs, setRuns] = useState<AutomationRun[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      try {
        const [a, r, c] = await Promise.all([
          getAutomation(id),
          listAutomationRuns(orgId, id),
          getAutomationCatalog(orgId),
        ])
        setItem(a)
        setRuns(r.items)
        setCatalog(c)
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
    triggerParams: item.trigger_params ?? {},
    conditions: flat.items,
    actions: item.actions ?? [],
  }) // read-only: no invented defaults

  const inboundId = item.trigger_params?.inbound_webhook_id
  const inboundName =
    inboundId &&
    (catalog?.inbound_webhooks?.find((h) => h.id === inboundId)?.name || inboundId)

  return (
    <section>
      <PageHeader title={item.name || 'Automation'} />
      <p className="lede">
        {item.enabled ? 'Enabled' : 'Disabled'} · trigger <code>{item.trigger}</code>
        {inboundName ? (
          <>
            {' '}
            · inbound <code>{inboundName}</code>
          </>
        ) : null}
      </p>
      <ShowActions
        editTo={resourcePath(orgId, 'automations', item.id, 'edit')}
        backTo={resourcePath(orgId, 'automations')}
      />
      <AutomationDag
        readOnly
        catalog={catalog}
        trigger={graph.trigger}
        triggerParams={graph.triggerParams}
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
    </section>
  )
}
