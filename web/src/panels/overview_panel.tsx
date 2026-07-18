import { Link } from 'react-router-dom'
import { orgPath, type OrgSection } from '../org_nav'

type Tile = {
  label: string
  value: string | number
  to: OrgSection
}

export function OverviewPanel({
  orgId,
  tiles,
}: {
  orgId: string
  tiles: Tile[]
}) {
  return (
    <section className="overview">
      <p className="lede">
        Jump into a section below, or use the sidebar. Switch organisations with the dropdown above.
      </p>
      <ul className="overview-stats">
        {tiles.map((tile) => (
          <li key={tile.to}>
            <Link className="overview-tile" to={orgPath(orgId, tile.to)}>
              <strong>{tile.value}</strong>
              <span>{tile.label}</span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
