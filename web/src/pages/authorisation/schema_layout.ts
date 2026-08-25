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

/**
 * An instance of a declared type: `editor` is one of `role`, `apollo` one of
 * `project`.
 */
export type SchemaInstance = { id: string; label: string }

export type SchemaInstanceType = {
  type: string
  instances: SchemaInstance[]
  total: number
  truncated: boolean
}

const INSTANCE_COLS = 4
const INSTANCE_COL = 148
const INSTANCE_ROW = 40
/** The gap between the last type column and the instance region. */
const INSTANCE_GUTTER = 120

/**
 * Instances go in their own region, to the right of every type.
 *
 * Placing them beside the type they belong to was the first attempt and it does
 * not survive a cap of a hundred: a block that tall pushes the type columns
 * apart until the model itself stops fitting on a screen, and the model is what
 * the page is for. A region of its own keeps the schema layout byte-identical to
 * what it was, so adding this could not change a picture anybody had already
 * learnt to read.
 *
 * Membership is carried by an edge to the type node rather than by adjacency.
 * A hundred thin arrows converging on `role` draw a fan, and a fan is the
 * correct picture: these are all the same kind of thing.
 *
 * Within a type the instances fill a grid rather than a column, because a
 * hundred nodes stacked vertically is nine screens and the same hundred in four
 * columns is two.
 */
export function layoutInstances(
  graph: SchemaGraph,
  types: SchemaInstanceType[],
): { nodes: ReturnType<typeof layout>; edges: SchemaEdge[] } {
  const depth = depths(graph.nodes, graph.edges)
  const maxDepth = Math.max(0, ...graph.nodes.map((n) => depth.get(n.id) ?? 0))
  const originX = (maxDepth + 1) * COLUMN + INSTANCE_GUTTER

  // Type name to the id core/reach gave its node, read from the graph rather
  // than recomputed here. Go sanitises those ids for Mermaid's parser, and a
  // second implementation of that rule on this side could drift from the first
  // and draw arrows to nodes that do not exist.
  //
  // It doubles as the filter. A row can outlive the declaration it was written
  // under — delete a capability and its resource type leaves the schema while
  // its edges stay — and a type absent from this map is absent from the canvas,
  // so its instances are skipped rather than pointed into nothing.
  const drawn = new Map(
    graph.nodes.filter((n) => n.kind === 'type').map((n) => [n.label, n.id] as const),
  )

  const nodes: ReturnType<typeof layout> = []
  const edges: SchemaEdge[] = []
  let y = 0

  for (const t of types) {
    const typeNode = drawn.get(t.type)
    if (!typeNode || t.instances.length === 0) continue

    // A header per run, so a reader can tell which fan is which without
    // following an arrow to its far end, and so the cap has somewhere to be
    // stated at the place it applies.
    nodes.push({
      id: `instances__${t.type}`,
      type: 'instanceGroup',
      position: { x: originX, y: y * INSTANCE_ROW },
      data: { group: t },
      draggable: true,
    } as never)
    y += 1

    t.instances.forEach((inst, i) => {
      const col = i % INSTANCE_COLS
      const row = Math.floor(i / INSTANCE_COLS)
      const id = `instance__${t.type}__${inst.id}`
      nodes.push({
        id,
        type: 'instance',
        position: { x: originX + col * INSTANCE_COL, y: (y + row) * INSTANCE_ROW },
        data: { instance: inst, type: t.type },
        draggable: true,
      } as never)
      // Type to instance, not the other way round. "editor is a role" is the
      // truer sentence, but the whole canvas flows left to right and the
      // instance region is on the right, so an arrow pointing back at the type
      // would leave its node on the wrong side and loop around it. Every other
      // edge here leaves a right handle and arrives at a left one.
      edges.push({
        id: `${id}__of`,
        from: typeNode,
        to: id,
        label: 'has',
        relations: ['instance'],
      })
    })

    y += Math.ceil(t.instances.length / INSTANCE_COLS) + 1
  }
  return { nodes, edges }
}
