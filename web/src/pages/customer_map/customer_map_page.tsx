import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  type Edge,
  type NodeProps,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { getMerchantSchema } from '../../api'
import { PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'
import { layout, type SchemaGraph, type SchemaNode } from '../authorisation/schema_layout'

/**
 * The organisation's own access model, drawn.
 *
 * Unlike KYC's map, this one belongs in the sidebar rather than the docs: it
 * renders something the merchant wrote and can change. The five pages beside it
 * declare the vocabulary; this is the shape those declarations add up to, which
 * was previously visible nowhere.
 *
 * The schema is derived from the vocabulary on every request rather than
 * stored, so it cannot drift from the objects it describes: declare a
 * capability and the type appears here immediately.
 *
 * It reuses the layout and node rendering built for the KYC map. One renderer,
 * because a schema is a schema, which is the property that made this page cost
 * almost nothing once the merchant tier was on the graph.
 */
export function CustomerMapPage() {
  const { orgId = '' } = useParams()
  const [graph, setGraph] = useState<SchemaGraph | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    void getMerchantSchema(orgId)
      .then((g) => live && setGraph(g as unknown as SchemaGraph))
      .catch((e) => live && setError(e instanceof Error ? e.message : 'Could not load the model'))
    return () => {
      live = false
    }
  }, [orgId])

  if (error) return <p className="error">{error}</p>
  if (!graph) return <p className="app-muted">Loading the model…</p>

  const empty = graph.summary.types <= 3

  return (
    <section>
      <PageHeader title="Map" />
      <p className="lede">
        What your declarations add up to. Scope kinds are containers, a
        capability&rsquo;s resource is a type and its action is what that type
        answers, and roles and groups are sets reached through membership.
      </p>

      {empty ? (
        <p className="app-muted schema-prose">
          Nothing to draw yet. Declare a{' '}
          <Link to={resourcePath(orgId, 'customer-scope-kinds')}>scope kind</Link> and a{' '}
          <Link to={resourcePath(orgId, 'customer-capabilities')}>capability</Link>, and the model
          appears here.
        </p>
      ) : (
        <>
          <SchemaSummary graph={graph} />
          <div className="schema-canvas">
            <ReactFlowProvider>
              <MapFlow graph={graph} />
            </ReactFlowProvider>
          </div>
          <p className="app-muted schema-caption">
            Drag a node to rearrange. Diamonds are not types you declared: they
            stand for a set of targets several relations share, drawn once so the
            picture shows the model rather than the repetition.
          </p>
        </>
      )}

      <h3 className="schema-heading">Reading a rule</h3>
      <p className="schema-prose">
        <code>read = can_read + parent-&gt;read</code> is why one grant on a
        container covers everything inside it. <code>can_read</code> is a grant
        written directly here; <code>parent-&gt;read</code> walks to whatever
        this sits in and asks again there.
      </p>
      <p className="schema-prose">
        That arrow is also why containment edges matter. A resource with no{' '}
        <code>parent</code> edge is reachable by nothing, because no walk can
        arrive at it. Write those from your own product with{' '}
        <Link to={orgPath(orgId, 'customer-edges')}>Edges</Link>, and try a
        question in the{' '}
        <Link to={orgPath(orgId, 'customer-playground')}>Playground</Link>.
      </p>
    </section>
  )
}

function SchemaSummary({ graph }: { graph: SchemaGraph }) {
  const s = graph.summary
  return (
    <dl className="schema-summary">
      <Stat label="types" value={s.types} />
      <Stat label="actions" value={s.actions} />
      <Stat label="relations" value={s.relations} />
      <Stat label="rules" value={s.rules} />
    </dl>
  )
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="schema-stat">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  )
}

function MapFlow({ graph }: { graph: SchemaGraph }) {
  const nodes = useMemo(() => layout(graph), [graph])
  const edges: Edge[] = useMemo(
    () =>
      graph.edges.map((e) => ({
        id: e.id,
        source: e.from,
        target: e.to,
        label: e.label,
        type: 'bezier',
      })),
    [graph],
  )
  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={mapNodeTypes}
      fitView
      fitViewOptions={{ padding: 0.15 }}
      nodesConnectable={false}
      panOnScroll
      zoomOnScroll
      minZoom={0.2}
      proOptions={{ hideAttribution: true }}
    >
      <Background gap={16} size={1} />
      <Controls showInteractive={false} />
      <MiniMap pannable zoomable />
    </ReactFlow>
  )
}

function MapNode({ data }: NodeProps) {
  const node = (data as { node: SchemaNode }).node
  const isSet = node.kind === 'set'
  return (
    <div className={isSet ? 'schema-node schema-node-set' : 'schema-node'}>
      <Handle type="target" position={Position.Left} />
      <p className="schema-node-name">{node.label}</p>
      {node.rules && node.rules.length > 0 && (
        <ul className="schema-node-rules">
          {node.rules.map((r) => (
            <li key={r.action}>
              <span className="schema-action">{r.action}</span>
              <span className="schema-eq">=</span>
              <span className="schema-expr">{r.expr}</span>
            </li>
          ))}
        </ul>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

const mapNodeTypes = { schema: MapNode }
