import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  checkMerchant,
  getMerchantSchema,
  listMerchantObjects,
  type MerchantDecision,
  type MerchantObjects,
} from '../../api'
import { PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'
import type { SchemaGraph } from '../authorisation/schema_layout'

/**
 * Ask the question your product asks, and see the answer with its route.
 *
 * Both directions live here because they are the same question asked two ways.
 * A check is what a request path calls; a listing is what a page calls, and
 * answering that one with a check per row is what makes an authorisation
 * service unusable at any real size.
 *
 * The route is the part worth having. A denial that only says "no" leaves
 * somebody guessing between four different fixes, and a graph is exactly the
 * kind of system where the answer is a path rather than a flag.
 */
export function CustomerPlaygroundPage() {
  const { orgId = '' } = useParams()
  const [schema, setSchema] = useState<SchemaGraph | null>(null)
  const [error, setError] = useState<string | null>(null)

  const [ask, setAsk] = useState({
    subject_id: '',
    action: '',
    resource_type: '',
    resource_id: '',
  })
  const [decision, setDecision] = useState<MerchantDecision | null>(null)
  const [objects, setObjects] = useState<MerchantObjects | null>(null)

  useEffect(() => {
    let live = true
    void getMerchantSchema(orgId)
      .then((g) => live && setSchema(g as unknown as SchemaGraph))
      .catch(() => live && setSchema(null))
    return () => {
      live = false
    }
  }, [orgId])

  const types = (schema?.nodes ?? []).filter((n) => n.kind === 'type').map((n) => n.label)
  const actions = new Set<string>()
  for (const n of schema?.nodes ?? []) {
    for (const r of n.rules ?? []) actions.add(r.action)
  }

  async function onCheck(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setObjects(null)
    try {
      setDecision(await checkMerchant(orgId, ask))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Check failed')
    }
  }

  async function onList() {
    setError(null)
    setDecision(null)
    try {
      setObjects(
        await listMerchantObjects(orgId, {
          subject_id: ask.subject_id,
          action: ask.action,
          resource_type: ask.resource_type,
        }),
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Listing failed')
    }
  }

  if (types.length === 0) {
    return (
      <section>
        <PageHeader title="Playground" />
        <p className="app-muted">
          Declare a <Link to={resourcePath(orgId, 'customer-scope-kinds')}>scope kind</Link> and a{' '}
          <Link to={resourcePath(orgId, 'customer-capabilities')}>capability</Link> first, then
          write some <Link to={orgPath(orgId, 'customer-edges')}>edges</Link>.
        </p>
      </section>
    )
  }

  return (
    <section>
      <PageHeader title="Playground" />
      <p className="lede">
        The call that replaces your authorisation layer. Ask about one resource,
        or ask what a customer can see across a whole type.
      </p>
      {error && <p className="error">{error}</p>}

      <form className="edge-form wide" onSubmit={onCheck}>
        <label>
          Customer
          <input
            value={ask.subject_id}
            onChange={(e) => setAsk({ ...ask, subject_id: e.target.value })}
            placeholder="an app user id"
            required
          />
        </label>
        <label>
          Action
          <select
            value={ask.action}
            onChange={(e) => setAsk({ ...ask, action: e.target.value })}
            required
          >
            <option value="">Choose…</option>
            {[...actions].sort().map((a) => (
              <option key={a}>{a}</option>
            ))}
          </select>
        </label>
        <label>
          Resource type
          <select
            value={ask.resource_type}
            onChange={(e) => setAsk({ ...ask, resource_type: e.target.value })}
            required
          >
            <option value="">Choose…</option>
            {types.map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </label>
        <label>
          Resource id
          <input
            value={ask.resource_id}
            onChange={(e) => setAsk({ ...ask, resource_id: e.target.value })}
            placeholder="required to check one; ignored when listing"
          />
          <span className="field-hint">
            Leave the id out and use <strong>List</strong> to ask what this customer can see across
            the whole type, which is what a page asks.
          </span>
        </label>
        <div className="form-actions">
          <button type="submit" className="button">
            Check
          </button>
          <button type="button" className="ghost" onClick={() => void onList()}>
            List
          </button>
        </div>
      </form>

      {decision && <Decision decision={decision} />}
      {objects && <Objects objects={objects} resourceType={ask.resource_type} />}
    </section>
  )
}

/**
 * Four answers, not two. A boolean flattens them, and the distinction is what
 * says which fix applies: no route at all, a route that grants something else,
 * or a rule that matched and was then subtracted from.
 */
const REASON: Record<MerchantDecision['reason'], string> = {
  allowed: 'allowed',
  unreachable: 'no route arrives',
  no_rule: 'a route arrives, but nothing grants this action',
  excluded: 'granted, then withdrawn by a subtraction',
}

function Decision({ decision }: { decision: MerchantDecision }) {
  return (
    <div className="access-panel">
      <div className={decision.allowed ? 'verdict ok' : 'verdict deny'}>
        <strong>{decision.allowed ? 'Allowed' : 'Denied'}</strong>
        <span>{REASON[decision.reason] ?? decision.reason}</span>
      </div>
      {decision.path.length === 0 ? (
        <p className="access-empty">
          Nothing was walked. For a denial that is the point: there was no route to
          take.
        </p>
      ) : (
        <ol className="access-path">
          {decision.path.map((hop, i) => (
            <li key={i} className="access-hop">
              <span className="access-node">{hop.object}</span>
              <span className="access-rel">#{hop.relation}</span>
              <span className="access-node">{hop.subject}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

function Objects({ objects, resourceType }: { objects: MerchantObjects; resourceType: string }) {
  return (
    <div className="access-panel">
      <p className="access-panel-sub">
        {objects.object_ids.length} {resourceType}
        {objects.object_ids.length === 1 ? '' : 's'} reachable
      </p>
      {objects.all && (
        <p className="app-muted">
          A <strong>wildcard grant</strong> covers every {resourceType}, including
          ones no edge names. Treat this list as a lower bound: filtering a page by
          it would hide rows this customer may in fact see.
        </p>
      )}
      {objects.truncated && (
        <p className="app-muted">
          The walk hit its bound, so this is a subset rather than the whole answer.
        </p>
      )}
      <ul className="reach-list">
        {objects.object_ids.map((id) => (
          <li key={id}>
            <code>
              {resourceType}:{id}
            </code>
          </li>
        ))}
      </ul>
    </div>
  )
}
