/**
 * Laying out a schema graph.
 *
 * Nodes are placed by how far they are from a sink, so what a type *depends on*
 * sits to its right and the principals every grant points at end up in one
 * column. That reads the way the schema does: an area type reaches an
 * organisation, and both are answered by somebody.
 *
 * This is deliberately not a general graph layout. A schema is shallow and
 * mostly fan-in, so ranking by depth and stacking within a rank is enough, and
 * a force simulation would move nodes between reloads for no gain.
 */

export type SchemaRule = { action: string; expr: string }

export type SchemaNode = {
  id: string
  kind: 'type' | 'set'
  label: string
  rules?: SchemaRule[]
  members?: string[]
}

export type SchemaEdge = {
  id: string
  from: string
  to: string
  label: string
  relations: string[]
}

/**
 * A set of types indistinguishable except by name.
 *
 * Computed in Go and shipped inside the artefact rather than derived here.
 * Two halves define it and both matter: thirteen of KYC's eighteen types hang
 * off `organisation` and look identical in the picture, but they answer four
 * different sets of actions, and only eight are genuinely one shape. A second
 * implementation on this side could disagree with the first, which is exactly
 * the kind of drift a generated file exists to prevent.
 */
export type SchemaShape = {
  types: string[]
  rules?: string[]
}

export type SchemaGraph = {
  namespace: string
  nodes: SchemaNode[]
  edges: SchemaEdge[]
  shapes?: SchemaShape[]
  summary: {
    namespace: string
    actions: number
    relations: number
    types: number
    rules: number
    wildcards: number
    transitive: number
  }
}

const COLUMN = 300
const ROW = 132

/**
 * Depth is the longest route from a node to a sink.
 *
 * A schema may contain a cycle: `group` whose `member_of` targets
 * `group#member_of` is the ordinary case, not a mistake. So the walk carries the
 * nodes currently on its stack and treats a re-entry as depth zero rather than
 * recursing forever.
 */
function depths(nodes: SchemaNode[], edges: SchemaEdge[]): Map<string, number> {
  const out = new Map<string, string[]>()
  for (const n of nodes) out.set(n.id, [])
  for (const e of edges) out.get(e.from)?.push(e.to)

  const depth = new Map<string, number>()
  const onStack = new Set<string>()

  const walk = (id: string): number => {
    const cached = depth.get(id)
    if (cached !== undefined) return cached
    if (onStack.has(id)) return 0
    onStack.add(id)
    let best = 0
    for (const next of out.get(id) ?? []) {
      best = Math.max(best, walk(next) + 1)
    }
    onStack.delete(id)
    depth.set(id, best)
    return best
  }

  for (const n of nodes) walk(n.id)
  return depth
}

/**
 * Position every node. Deepest nodes go left, sinks right, and each column is
 * stacked in the order the graph lists them, which is already sorted, so the
 * picture is stable across reloads.
 */
export function layout(graph: SchemaGraph) {
  const depth = depths(graph.nodes, graph.edges)
  const maxDepth = Math.max(0, ...graph.nodes.map((n) => depth.get(n.id) ?? 0))
  const filled = new Map<number, number>()

  return graph.nodes.map((n) => {
    const d = depth.get(n.id) ?? 0
    const row = filled.get(d) ?? 0
    filled.set(d, row + 1)
    return {
      id: n.id,
      type: 'schema',
      position: { x: (maxDepth - d) * COLUMN, y: row * ROW },
      data: { node: n },
      draggable: true,
    }
  })
}
