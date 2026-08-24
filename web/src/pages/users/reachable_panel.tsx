import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getMerchantSchema, listMerchantObjects, type MerchantObjects } from '../../api'
import { orgPath } from '../../org_nav'
import type { SchemaGraph } from '../authorisation/schema_layout'

/**
 * What this customer can actually reach, resource by resource.
 *
 * The grant list above says what they were given; this says what that adds up
 * to. They are different questions, and only the second one answers "why can
 * they open that document?", because a grant on a container never names the
 * things inside it.
 *
 * One query per type rather than a check per object. That is the difference
 * between a panel that renders and one that cannot exist: a customer on a
 * container holding ten thousand documents would otherwise be ten thousand
 * walks.
 */
export function ReachablePanel({ appUserId }: { appUserId: string }) {
  const { orgId = '' } = useParams()
  const [schema, setSchema] = useState<SchemaGraph | null>(null)
  const [results, setResults] = useState<Record<string, MerchantObjects>>({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let live = true
    void getMerchantSchema(orgId)
      .then((g) => live && setSchema(g as unknown as SchemaGraph))
      .catch(() => live && setSchema(null))
      .finally(() => live && setLoading(false))
    return () => {
      live = false
    }
  }, [orgId])

  // Structural types carry membership rather than permission, so asking what a
  // customer can "read" about a role is a question with no meaning.
  const structural = new Set(['app_user', 'group', 'role'])
  const askable: { type: string; actions: string[] }[] = []
  for (const node of schema?.nodes ?? []) {
    if (node.kind !== 'type' || structural.has(node.label)) continue
    const actions = (node.rules ?? []).map((r) => r.action)
    if (actions.length > 0) askable.push({ type: node.label, actions })
  }

  async function load(type: string, action: string) {
    const key = `${type}:${action}`
    try {
      const out = await listMerchantObjects(orgId, {
        subject_id: appUserId,
        action,
        resource_type: type,
      })
      setResults((prev) => ({ ...prev, [key]: out }))
    } catch {
      // A failure here must not take the page with it: the grant list above is
      // still the more important half.
    }
  }

  if (loading || askable.length === 0) return null

  return (
    <>
      <h3 className="section-title">What they can reach</h3>
      <p className="muted">
        The list above is what they were granted. This is what it adds up to,
        including things reached through a container rather than named directly.
        Written from your product with{' '}
        <Link to={orgPath(orgId, 'customer-edges')}>Edges</Link>.
      </p>
      <div className="reachable-grid">
        {askable.map(({ type, actions }) =>
          actions.map((action) => {
            const key = `${type}:${action}`
            const got = results[key]
            return (
              <div key={key} className="reachable-card">
                <div className="reachable-head">
                  <code>
                    {type} &middot; {action}
                  </code>
                  {!got && (
                    <button type="button" className="link-btn" onClick={() => void load(type, action)}>
                      Show
                    </button>
                  )}
                </div>
                {got && (
                  <>
                    <p className="reachable-count">
                      {got.all
                        ? `every ${type}`
                        : `${got.object_ids.length} ${got.object_ids.length === 1 ? 'item' : 'items'}`}
                      {got.truncated && ' (subset)'}
                    </p>
                    {got.object_ids.length > 0 && (
                      <ul className="reach-list">
                        {got.object_ids.slice(0, 20).map((id) => (
                          <li key={id}>{id}</li>
                        ))}
                      </ul>
                    )}
                  </>
                )}
              </div>
            )
          }),
        )}
      </div>
    </>
  )
}
