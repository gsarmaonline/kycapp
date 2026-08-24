import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  deleteMerchantEdge,
  getMerchantSchema,
  listMerchantSubjects,
  writeMerchantEdges,
  type MerchantEdge,
} from '../../api'
import { PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'
import type { SchemaGraph } from '../authorisation/schema_layout'

/**
 * The edges: what a merchant's product owns and contains.
 *
 * This is the object the rest of customer access exists to serve, and it had no
 * page at all. Scope kinds, capabilities, roles and grants declare a
 * vocabulary; an edge is a fact, and facts are what the walk actually reads.
 *
 * Most edges are written by a merchant's backend on every resource create, not
 * typed in here. This page is for the two things a form is better at than an
 * API call: seeing what is really stored when a check surprises someone, and
 * writing one fact by hand to find out why.
 *
 * There is deliberately no list of every edge. The table is unbounded, and a
 * page that tried would be a scan of the merchant's whole product. You ask
 * about an object instead, which is the same question a share dialog asks.
 */
export function CustomerEdgesPage() {
  const { orgId = '' } = useParams()
  const [schema, setSchema] = useState<SchemaGraph | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const [form, setForm] = useState<MerchantEdge>({
    object_type: '',
    object_id: '',
    relation: '',
    subject_type: 'app_user',
    subject_id: '',
    subject_relation: '',
  })

  const [lookup, setLookup] = useState({ resource_type: '', resource_id: '', action: '' })
  const [reached, setReached] = useState<{ type: string; id: string }[] | null>(null)
  const [reachedAll, setReachedAll] = useState(false)

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
  const relations = new Set<string>()
  for (const e of schema?.edges ?? []) {
    for (const r of e.relations) relations.add(r)
  }
  const actions = new Set<string>()
  for (const n of schema?.nodes ?? []) {
    for (const r of n.rules ?? []) actions.add(r.action)
  }

  async function onWrite(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setNotice(null)
    try {
      await writeMerchantEdges(orgId, [form])
      setNotice(`Wrote ${form.object_type}:${form.object_id} #${form.relation}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Write failed')
    }
  }

  async function onDelete() {
    setError(null)
    setNotice(null)
    try {
      await deleteMerchantEdge(orgId, form)
      setNotice('Deleted, if it was there. Deleting a fact that is absent is not an error.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  async function onLookup(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setReached(null)
    try {
      const out = await listMerchantSubjects(orgId, lookup)
      setReached(out.subjects)
      setReachedAll(out.all)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Lookup failed')
    }
  }

  if (types.length === 0) {
    return (
      <section>
        <PageHeader title="Edges" />
        <p className="app-muted">
          Declare a <Link to={resourcePath(orgId, 'customer-scope-kinds')}>scope kind</Link> and a{' '}
          <Link to={resourcePath(orgId, 'customer-capabilities')}>capability</Link> first. An edge
          can only name types your model has.
        </p>
      </section>
    )
  }

  return (
    <section>
      <PageHeader title="Edges" />
      <p className="lede">
        The facts your product states about itself: who owns what, and what sits
        inside what. Your backend writes these on every create and delete. This
        page is for checking what is really stored, and writing one by hand when
        a decision surprises you.
      </p>
      {error && <p className="error">{error}</p>}
      {notice && <p className="app-muted">{notice}</p>}

      <h3 className="schema-heading">Who reaches an object</h3>
      <p className="schema-prose">
        There is no list of every edge, on purpose: that table is your whole
        product, and a page that tried to show it would be a scan. Ask about one
        object instead.
      </p>
      <form className="edge-form" onSubmit={onLookup}>
        <label>
          Type
          <select
            value={lookup.resource_type}
            onChange={(e) => setLookup({ ...lookup, resource_type: e.target.value })}
            required
          >
            <option value="">Choose…</option>
            {types.map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </label>
        <label>
          Id
          <input
            value={lookup.resource_id}
            onChange={(e) => setLookup({ ...lookup, resource_id: e.target.value })}
            required
          />
        </label>
        <label>
          Action
          <select
            value={lookup.action}
            onChange={(e) => setLookup({ ...lookup, action: e.target.value })}
            required
          >
            <option value="">Choose…</option>
            {[...actions].sort().map((a) => (
              <option key={a}>{a}</option>
            ))}
          </select>
        </label>
        <button type="submit" className="button">
          Look up
        </button>
      </form>

      {reached && (
        <div className="edge-result">
          {reachedAll && (
            <p className="app-muted">
              An <strong>everyone</strong> grant reaches this, so the list below is
              a lower bound rather than the answer.
            </p>
          )}
          {reached.length === 0 && !reachedAll ? (
            <p className="app-muted">Nobody reaches it.</p>
          ) : (
            <ul className="reach-list">
              {reached.map((s) => (
                <li key={`${s.type}:${s.id}`}>
                  <code>
                    {s.type}:{s.id}
                  </code>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <h3 className="schema-heading">Write one fact</h3>
      <p className="schema-prose">
        <code>document:d1 #parent project:apollo</code> says d1 sits in apollo.
        Writes are idempotent, so stating a fact twice is safe.
      </p>
      <form className="edge-form wide" onSubmit={onWrite}>
        <label>
          Object type
          <select
            value={form.object_type}
            onChange={(e) => setForm({ ...form, object_type: e.target.value })}
            required
          >
            <option value="">Choose…</option>
            {types.map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </label>
        <label>
          Object id
          <input
            value={form.object_id}
            onChange={(e) => setForm({ ...form, object_id: e.target.value })}
            placeholder="d1, or * for every one"
            required
          />
        </label>
        <label>
          Relation
          <select
            value={form.relation}
            onChange={(e) => setForm({ ...form, relation: e.target.value })}
            required
          >
            <option value="">Choose…</option>
            {[...relations].sort().map((r) => (
              <option key={r}>{r}</option>
            ))}
          </select>
        </label>
        <label>
          Subject type
          <select
            value={form.subject_type}
            onChange={(e) => setForm({ ...form, subject_type: e.target.value })}
            required
          >
            {types.map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </label>
        <label>
          Subject id
          <input
            value={form.subject_id}
            onChange={(e) => setForm({ ...form, subject_id: e.target.value })}
            required
          />
        </label>
        <label>
          Subject relation
          <input
            value={form.subject_relation}
            onChange={(e) => setForm({ ...form, subject_relation: e.target.value })}
            placeholder="holder, for a role. Blank for a plain node."
          />
          <span className="field-hint">
            Set this to point at a set rather than one node: <code>role:editor#holder</code> is
            whoever holds that role.
          </span>
        </label>
        <div className="form-actions">
          <button type="submit" className="button">
            Write
          </button>
          <button type="button" className="ghost" onClick={() => void onDelete()}>
            Delete
          </button>
        </div>
      </form>
    </section>
  )
}
