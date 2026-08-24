import { describe, expect, it } from 'vitest'
import { layout, type SchemaGraph } from './schema_layout'

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
