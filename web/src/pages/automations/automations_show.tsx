import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getAutomation, listAutomationRuns, type Automation, type AutomationRun } from '../../api'
import { DetailList, PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'

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

  return (
    <section>
      <PageHeader title="Automation" />
      <DetailList
        items={[
          { label: 'Name', value: item.name || '—' },
          { label: 'Trigger', value: item.trigger },
          { label: 'Enabled', value: item.enabled ? 'yes' : 'no' },
          {
            label: 'Conditions',
            value:
              item.conditions?.all?.length
                ? item.conditions.all
                    .map((c) => `${c.field} ${c.op} ${c.value ?? ''}`.trim())
                    .join('; ')
                : '(none)',
          },
          {
            label: 'Actions',
            value: item.actions.map((a) => `${a.type}:${a.template_key ?? ''}`).join(', '),
          },
        ]}
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
