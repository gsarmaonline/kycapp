import { useEffect, useMemo, useState } from 'react'
import {
  explainMembershipAccess,
  type AccessExplanation,
  type PermissionOutcome,
} from '../api'

/**
 * Why this member can or cannot do each thing.
 *
 * The engine records the route it took to every decision, so this shows a route
 * rather than a boolean. That is the whole reason a graph is worth walking: a
 * flat check can only answer "the row says so", while a path answers "through
 * which role, inherited from where".
 *
 * It lives on the member page rather than behind its own nav entry because the
 * question is always asked about somebody. A destination would mean navigating
 * away from the member in order to ask about that member.
 */
export function AccessPathPanel({ membershipId }: { membershipId: string }) {
  const [data, setData] = useState<AccessExplanation | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [open, setOpen] = useState<string | null>(null)
  const [denied, setDenied] = useState(false)

  useEffect(() => {
    let live = true
    void explainMembershipAccess(membershipId)
      .then((d) => live && setData(d))
      .catch((e) => live && setError(e instanceof Error ? e.message : 'Could not load access'))
    return () => {
      live = false
    }
  }, [membershipId])

  const groups = useMemo(() => groupByResource(data?.outcomes ?? [], denied), [data, denied])
  const allowed = data?.outcomes.filter((o) => o.allowed).length ?? 0
  const total = data?.outcomes.length ?? 0

  if (error) return <p className="error">{error}</p>
  if (!data) return <p className="app-muted">Loading access…</p>

  return (
    <section className="access-panel">
      <header className="access-panel-head">
        <div>
          <h2 className="access-panel-title">Access</h2>
          <p className="access-panel-sub">
            {allowed} of {total} permissions, each with the route the graph took
          </p>
        </div>
        <label className="access-toggle">
          <input
            type="checkbox"
            checked={denied}
            onChange={(e) => setDenied(e.target.checked)}
          />
          Show denied
        </label>
      </header>

      {groups.length === 0 ? (
        <p className="app-muted">
          Nothing granted here. Turn on <strong>Show denied</strong> to see why each
          permission was refused.
        </p>
      ) : (
        groups.map(([resource, outcomes]) => (
          <div key={resource} className="access-group">
            <h3 className="access-group-title">{resource.replace(/_/g, ' ')}</h3>
            <ul className="access-rows">
              {outcomes.map((o) => (
                <li key={o.key}>
                  <button
                    type="button"
                    className="access-row"
                    aria-expanded={open === o.key}
                    onClick={() => setOpen(open === o.key ? null : o.key)}
                  >
                    <span className="access-action">{actionOf(o.key)}</span>
                    <ReasonChip outcome={o} />
                    <span className="access-hops">
                      {o.path.length > 0 ? `${o.path.length} hop${o.path.length > 1 ? 's' : ''}` : '—'}
                    </span>
                  </button>
                  {open === o.key && <Path outcome={o} />}
                </li>
              ))}
            </ul>
          </div>
        ))
      )}
    </section>
  )
}

/**
 * The four answers the engine distinguishes. A boolean would flatten them, and
 * the distinction is what tells you what to do next: `unreachable` means no
 * route arrives at all, `no_rule` means one arrives and grants something else,
 * and `excluded` means a rule matched and a subtraction removed it.
 */
const REASON_LABEL: Record<PermissionOutcome['reason'], string> = {
  allowed: 'allowed',
  unreachable: 'no route',
  no_rule: 'not granted',
  excluded: 'withdrawn',
}

function ReasonChip({ outcome }: { outcome: PermissionOutcome }) {
  return (
    <span className={`access-chip access-chip-${outcome.reason}`}>
      {REASON_LABEL[outcome.reason] ?? outcome.reason}
    </span>
  )
}

function Path({ outcome }: { outcome: PermissionOutcome }) {
  if (outcome.path.length === 0) {
    return (
      <p className="access-empty">
        {outcome.reason === 'unreachable'
          ? 'No route of any kind arrives here, so this member cannot see that it exists.'
          : outcome.reason === 'excluded'
            ? 'A rule matched and a subtraction removed it. Organisation status is the usual cause.'
            : 'A route arrives, but no rule here grants this action.'}
      </p>
    )
  }
  return (
    <ol className="access-path">
      {outcome.path.map((hop, i) => (
        <li key={i} className={hop.redacted ? 'access-hop access-hop-redacted' : 'access-hop'}>
          <span className="access-node">{hop.object}</span>
          <span className="access-rel">#{hop.relation}</span>
          <span className="access-node">{hop.subject}</span>
        </li>
      ))}
    </ol>
  )
}

function actionOf(key: string) {
  return key.split(':')[1] ?? key
}

function resourceOf(key: string) {
  return key.split(':')[0] ?? key
}

/**
 * Grouped by resource, because a permission key already reads that way and it
 * is how somebody scans for the thing they are actually asking about.
 */
function groupByResource(outcomes: PermissionOutcome[], includeDenied: boolean) {
  const keep = includeDenied ? outcomes : outcomes.filter((o) => o.allowed)
  const byResource = new Map<string, PermissionOutcome[]>()
  for (const o of keep) {
    const r = resourceOf(o.key)
    const list = byResource.get(r)
    if (list) list.push(o)
    else byResource.set(r, [o])
  }
  return [...byResource.entries()]
}
