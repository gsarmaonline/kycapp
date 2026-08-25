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
 *
 * `detail` is what the instance carries, where carrying something is what
 * separates one from the next. A role's own capabilities are the case that
 * matters: three roles with no detail and no edges are three identical chips,
 * and the picture then asserts they are interchangeable.
 */
export type SchemaInstance = { id: string; label: string; detail?: string[] }

export type SchemaInstanceType = {
  type: string
  instances: SchemaInstance[]
  total: number
  truncated: boolean
}

/** One fact relating two drawn instances: `admin extends member`. */
export type SchemaInstanceEdge = {
  from_type: string
  from_id: string
  label: string
  to_type: string
  to_id: string
}

const INSTANCE_COL = 168
const INSTANCE_ROW = 46
/** How far a run stacks before it starts a further column. */
const INSTANCE_ROWS_PER_COL = 10
/** The gap between the last type column and the instance region. */
const INSTANCE_GUTTER = 140

export function instanceNodeID(type: string, id: string) {
  return `instance__${type}__${id}`
}

export function instanceGroupID(type: string) {
  return `instances__${type}`
}

/**
 * Instances go in their own region, to the right of every type.
 *
 * Placing them beside the type they belong to does not survive a cap of a
 * hundred: a block that tall pushes the type columns apart until the model
 * itself stops fitting on a screen, and the model is what the page is for. A
 * region of its own keeps the schema layout byte-identical, so this could not
 * change a picture somebody had already learnt to read.
 *
 * Only one arrow crosses between the two regions, from the type node to the run
 * header. The first version drew one per instance, and that was the bug: a
 * hundred arrows saying "is a role" carry no information the header does not
 * already carry, and worse, they chained every instance into the schema's own
 * nodes so the picture read as though roles flowed into the model. What
 * separates member from admin is an edge between *them*, and those are the
 * edges this now draws.
 *
 * Within a run, position is by depth in the instance graph rather than by name.
 * A base role sits to the right of the role that extends it, matching the rest
 * of the canvas, where an arrow always leaves a right handle and arrives at a
 * left one. That is the entire reason to draw the relations at all: the shape
 * of the run is the shape of the inheritance.
 */
export function layoutInstances(
  graph: SchemaGraph,
  types: SchemaInstanceType[],
  instanceEdges: SchemaInstanceEdge[] = [],
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
  const typeNodeOf = new Map(
    graph.nodes.filter((n) => n.kind === 'type').map((n) => [n.label, n.id] as const),
  )

  const drawnTypes = types.filter(
    (t) => typeNodeOf.has(t.type) && t.instances.length > 0,
  )

  const present = new Set<string>()
  for (const t of drawnTypes) {
    for (const inst of t.instances) present.add(instanceNodeID(t.type, inst.id))
  }

  // Relations first, because the layout is derived from them. An edge naming a
  // node the cap left out is dropped rather than drawn at a node that is not
  // there.
  const edges: SchemaEdge[] = []
  for (const e of instanceEdges) {
    const from = instanceNodeID(e.from_type, e.from_id)
    const to = instanceNodeID(e.to_type, e.to_id)
    if (!present.has(from) || !present.has(to)) continue
    edges.push({
      id: `${from}__${e.label}__${to}`,
      from,
      to,
      label: e.label,
      relations: [e.label],
    })
  }

  const instanceDepth = depths(
    [...present].map((id) => ({ id, kind: 'type' as const, label: id })),
    edges,
  )

  const nodes: ReturnType<typeof layout> = []
  let y = 0

  for (const t of drawnTypes) {
    const groupID = instanceGroupID(t.type)
    nodes.push({
      id: groupID,
      type: 'instanceGroup',
      position: { x: originX, y: y * INSTANCE_ROW },
      data: { group: t },
      draggable: true,
    } as never)

    // The single arrow between the two regions. It anchors the run to the type
    // it belongs to without asserting anything about the instances inside it.
    edges.push({
      id: `${groupID}__of`,
      from: typeNodeOf.get(t.type) as string,
      to: groupID,
      label: 'has',
      relations: ['instance'],
    })
    y += 1

    // Bucket by depth, deepest first, so the most derived instance is leftmost
    // and every arrow runs left to right like the rest of the canvas.
    const buckets = new Map<number, SchemaInstance[]>()
    for (const inst of t.instances) {
      const d = instanceDepth.get(instanceNodeID(t.type, inst.id)) ?? 0
      const bucket = buckets.get(d)
      if (bucket) bucket.push(inst)
      else buckets.set(d, [inst])
    }

    let col = 0
    let tallest = 0
    for (const d of [...buckets.keys()].sort((a, b) => b - a)) {
      const bucket = buckets.get(d) as SchemaInstance[]
      // A bucket taller than the run allows spills into further columns of its
      // own. Without this a type with no relations at all — every instance at
      // depth zero — would be one column a hundred rows tall.
      bucket.forEach((inst, i) => {
        const sub = Math.floor(i / INSTANCE_ROWS_PER_COL)
        const row = i % INSTANCE_ROWS_PER_COL
        nodes.push({
          id: instanceNodeID(t.type, inst.id),
          type: 'instance',
          position: {
            x: originX + (col + sub) * INSTANCE_COL,
            y: (y + row) * INSTANCE_ROW,
          },
          data: { instance: inst, type: t.type },
          draggable: true,
        } as never)
      })
      col += Math.ceil(bucket.length / INSTANCE_ROWS_PER_COL)
      tallest = Math.max(tallest, Math.min(bucket.length, INSTANCE_ROWS_PER_COL))
    }

    y += tallest + 1
  }
  return { nodes, edges }
}
