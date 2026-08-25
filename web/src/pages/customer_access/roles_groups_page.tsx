import { CustomerRolesIndex } from '../customer_roles/customer_roles_index'
import { CustomerGroupsIndex } from '../customer_groups/customer_groups_index'
import { ConceptDocsLink } from '../../components/ConceptDocsLink'

/**
 * Roles and groups: what named sets do we have.
 *
 * Two sidebar items for one mechanism. A role is a named set that confers
 * through membership and nests; a group is a named set that confers through
 * membership and nests. They have been the same thing since groups gained
 * nesting, and in the graph they are literally the same shape — role#holder and
 * group#member_of, both usersets the walk resolves.
 *
 * Two sections on one page rather than two pages, because presenting them as
 * separate concepts is what makes a merchant ask which one they are supposed to
 * use.
 */
export function CustomerRolesGroupsPage() {
  return (
    <div className="stack">
      <header className="page-intro">
        <h1>Roles &amp; groups</h1>
        <p className="muted">
          Two ways of naming a set. A role names a set of capabilities; a group names a set of
          customers. Both nest, and both confer through membership.{' '}
          <ConceptDocsLink slug="customer-roles" label="Roles and groups" />
        </p>
      </header>

      <section className="model-section">
        <CustomerRolesIndex />
      </section>

      <section className="model-section">
        <CustomerGroupsIndex />
      </section>
    </div>
  )
}
