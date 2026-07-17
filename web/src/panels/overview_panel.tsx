import type { Organisation } from '../api'

export function OverviewPanel({
  org,
  memberCount,
  roleCount,
}: {
  org: Organisation
  memberCount: number
  roleCount: number
}) {
  return (
    <section className="overview">
      <p className="lede">
        Use the sidebar to open Members, Roles, Users, User Attributes, Email templates, or Billing.
        Switch organisations with the dropdown above.
      </p>
      <ul className="overview-stats">
        <li>
          <strong>{memberCount}</strong>
          <span>Members</span>
        </li>
        <li>
          <strong>{roleCount}</strong>
          <span>Roles</span>
        </li>
        <li>
          <strong>{org.status}</strong>
          <span>Status</span>
        </li>
      </ul>
    </section>
  )
}
