import { describe, expect, it } from 'vitest'
import {
  layout,
  layoutInstances,
  type SchemaGraph,
  type SchemaInstanceType,
} from './schema_layout'

/**
 * Only the layout is tested here, because only the layout lives here. Which
 * types repeat is computed in Go and shipped inside the artefact, and
 * TestRepeatedShapesStayAtEight pins the answer against the real schema.
 */
function graphOf(
  nodes: SchemaGraph['nodes'],
  edges: Array<[string, string]>,
): SchemaGraph {
  return {
    namespace: 'test',
    nodes,
    edges: edges.map(([from, to]) => ({
      id: `${from}_${to}`,
      from,
      to,
      label: 'rel',
      relations: ['rel'],
    })),
    summary: {
      namespace: 'test',
      actions: 0,
      relations: 0,
      types: nodes.length,
      rules: 0,
      wildcards: 0,
      transitive: 0,
    },
  }
}

const chain = graphOf(
  [
    { id: 'area', kind: 'type', label: 'area' },
    { id: 'org', kind: 'type', label: 'org' },
    { id: 'principals', kind: 'set', label: 'user · role', members: ['role', 'user'] },
  ],
  [
    ['area', 'org'],
    ['area', 'principals'],
    ['org', 'principals'],
  ],
)

describe('layout', () => {
  it('positions every node exactly once', () => {
    const placed = layout(chain)
    expect(placed).toHaveLength(3)
    expect(new Set(placed.map((n) => n.id)).size).toBe(3)
  })

  it('is stable across calls', () => {
    // The picture must not move between reloads. Ranking by depth and stacking
    // in the graph's own sorted order is what buys that.
    expect(layout(chain)).toEqual(layout(chain))
  })

  it('puts what a type depends on to its right', () => {
    const at = new Map(layout(chain).map((n) => [n.id, n.position.x]))
    expect(at.get('area')!).toBeLessThan(at.get('org')!)
    expect(at.get('org')!).toBeLessThan(at.get('principals')!)
  })

  it('never overlaps two nodes', () => {
    const seen = new Set<string>()
    for (const n of layout(chain)) {
      const at = `${n.position.x},${n.position.y}`
      expect(seen.has(at), `${n.id} sits on another node`).toBe(false)
      seen.add(at)
    }
  })

  it('terminates on a schema containing a cycle', () => {
    // group#member_of targeting group is ordinary, not a mistake, so the walk
    // has to survive it rather than recurse forever.
    const cyclic = graphOf(
      [
        { id: 'a', kind: 'type', label: 'a' },
        { id: 'b', kind: 'type', label: 'b' },
      ],
      [
        ['a', 'b'],
        ['b', 'a'],
      ],
    )
    expect(layout(cyclic)).toHaveLength(2)
  })

  it('handles a node with no edges at all', () => {
    const lone = graphOf([{ id: 'user', kind: 'type', label: 'user' }], [])
    const placed = layout(lone)
    expect(placed).toHaveLength(1)
    expect(placed[0].position).toEqual({ x: 0, y: 0 })
  })
})

/**
 * The instance layer.
 *
 * The properties worth pinning are the ones a careless change breaks silently:
 * an arrow to a type node that is not on the canvas draws nothing and reports
 * nothing, and an instance region that overlaps the model corrupts the picture
 * the page exists for.
 */
describe('layoutInstances', () => {
  const withInstances = graphOf(
    [
      { id: 't_document', kind: 'type', label: 'document' },
      { id: 't_role', kind: 'type', label: 'role' },
    ],
    [['t_document', 't_role']],
  )

  const roles: SchemaInstanceType = {
    type: 'role',
    instances: [
      { id: 'app_role_1', label: 'admin' },
      { id: 'app_role_2', label: 'editor' },
    ],
    total: 2,
    truncated: false,
  }

  it('anchors every instance edge on the id its type node really has', () => {
    const { edges } = layoutInstances(withInstances, [roles])
    expect(edges).toHaveLength(2)
    for (const e of edges) {
      expect(withInstances.nodes.map((n) => n.id)).toContain(e.from)
    }
  })

  it('runs the instance edge the way the rest of the canvas flows', () => {
    // Left to right: out of the type node, into the instance on its right.
    const { nodes, edges } = layoutInstances(withInstances, [roles])
    const byId = new Map(nodes.map((n) => [n.id, n.position.x]))
    for (const e of edges) {
      expect(byId.get(e.to)).toBeDefined()
    }
  })

  it('skips a type the schema no longer draws', () => {
    // A capability is deleted, its resource type leaves the schema, and its
    // edges stay behind. The rows outlive the declaration.
    const orphan: SchemaInstanceType = {
      type: 'invoice',
      instances: [{ id: 'i1', label: 'i1' }],
      total: 1,
      truncated: false,
    }
    const { nodes, edges } = layoutInstances(withInstances, [orphan])
    expect(nodes).toHaveLength(0)
    expect(edges).toHaveLength(0)
  })

  it('keeps the instance region clear of every type node', () => {
    const { nodes } = layoutInstances(withInstances, [roles])
    const modelRight = Math.max(...layout(withInstances).map((n) => n.position.x))
    for (const n of nodes) {
      expect(n.position.x).toBeGreaterThan(modelRight)
    }
  })

  it('leaves the model layout untouched', () => {
    // Adding the instance layer must not move a picture somebody already reads.
    const before = layout(withInstances)
    layoutInstances(withInstances, [roles])
    expect(layout(withInstances)).toEqual(before)
  })

  it('wraps a long run into a grid rather than one tall column', () => {
    const many: SchemaInstanceType = {
      type: 'role',
      instances: Array.from({ length: 100 }, (_, i) => ({
        id: `r${i}`,
        label: `role-${i}`,
      })),
      total: 100,
      truncated: false,
    }
    const { nodes } = layoutInstances(withInstances, [many])
    const instanceNodes = nodes.filter((n) => n.id.startsWith('instance__'))
    expect(instanceNodes).toHaveLength(100)
    expect(new Set(instanceNodes.map((n) => n.position.x)).size).toBeGreaterThan(1)
  })
})
