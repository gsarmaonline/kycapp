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
import { getMerchantInstances, getMerchantSchema, type MerchantInstances } from '../../api'
import { PageHeader } from '../../crud/ui'
import { orgPath, resourcePath } from '../../org_nav'
import {
  layout,
  layoutInstances,
  type SchemaGraph,
  type SchemaInstance,
  type SchemaInstanceType,
  type SchemaNode,
} from '../authorisation/schema_layout'

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
  const [instances, setInstances] = useState<MerchantInstances | null>(null)
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

  // The instance layer fails quietly and separately. It is an addition to the
  // picture, and a merchant who may read the model but not list their customers
  // should still get the model rather than an error where the map was.
  useEffect(() => {
    let live = true
    void getMerchantInstances(orgId)
      .then((i) => live && setInstances(i))
      .catch(() => live && setInstances(null))
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
              <MapFlow graph={graph} instances={instances?.types ?? []} />
            </ReactFlowProvider>
          </div>
          <p className="app-muted schema-caption">
            Drag a node to rearrange. Diamonds are not types you declared: they
            stand for a set of targets several relations share, drawn once so the
            picture shows the model rather than the repetition. To the right of
            the model sits what you actually have: <code>editor</code> is one{' '}
            <code>role</code>, <code>apollo</code> one <code>project</code>.
          </p>
          <CapNotice instances={instances} orgId={orgId} />
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

function MapFlow({
  graph,
  instances,
}: {
  graph: SchemaGraph
  instances: SchemaInstanceType[]
}) {
  const placed = useMemo(() => layoutInstances(graph, instances), [graph, instances])
  const nodes = useMemo(() => [...layout(graph), ...placed.nodes], [graph, placed])
  const edges: Edge[] = useMemo(
    () => [
      ...graph.edges.map((e) => ({
        id: e.id,
        source: e.from,
        target: e.to,
        label: e.label,
        type: 'bezier',
      })),
      // Instance arrows carry no label. There are up to a hundred per type and
      // they all say the same thing, so the label would be the only text on the
      // canvas repeated a hundred times, and the fan already says it once.
      ...placed.edges.map((e) => ({
        id: e.id,
        source: e.from,
        target: e.to,
        type: 'bezier',
        className: 'schema-edge-instance',
      })),
    ],
    [graph, placed],
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

/**
 * The header of one type's fan.
 *
 * It states the cap where the cap applies. A count in prose under the canvas
 * would be read after the picture has already been believed, and the thing that
 * has to be disbelieved is a specific block of a hundred nodes.
 */
function InstanceGroupNode({ data }: NodeProps) {
  const group = (data as { group: SchemaInstanceType }).group
  return (
    <div className="schema-instance-group">
      <Handle type="target" position={Position.Left} />
      <p className="schema-instance-group-name">{group.type}</p>
      <span className="schema-instance-group-count">
        {group.truncated
          ? `${group.instances.length} of ${group.total.toLocaleString()}`
          : `${group.total.toLocaleString()} total`}
      </span>
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

/** One thing that exists: a role, a group, a customer, a project, a document. */
function InstanceNode({ data }: NodeProps) {
  const { instance } = data as { instance: SchemaInstance; type: string }
  return (
    <div className="schema-instance" title={instance.id}>
      <Handle type="target" position={Position.Left} />
      <span className="schema-instance-label">{instance.label}</span>
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

const mapNodeTypes = {
  schema: MapNode,
  instance: InstanceNode,
  instanceGroup: InstanceGroupNode,
}

/**
 * What the canvas is not showing.
 *
 * Only types over the cap appear. A notice that lists every type whether or not
 * it was trimmed trains a reader to skip it, and then it is not there on the day
 * a type does get trimmed.
 */
function CapNotice({
  instances,
  orgId,
}: {
  instances: MerchantInstances | null
  orgId: string
}) {
  const over = instances?.types.filter((t) => t.truncated) ?? []
  if (!instances || over.length === 0) return null
  return (
    <p className="schema-cap-notice">
      The map draws at most {instances.cap} of each type, so{' '}
      {over.map((t, i) => (
        <span key={t.type}>
          {i > 0 && (i === over.length - 1 ? ' and ' : ', ')}
          <code>{t.type}</code> is showing {t.instances.length} of{' '}
          {t.total.toLocaleString()}
        </span>
      ))}
      . A picture of every one of them answers nothing, so the rest are not
      drawn. To ask about a specific one, use the{' '}
      <Link to={orgPath(orgId, 'customer-playground')}>Playground</Link>.
    </p>
  )
}
