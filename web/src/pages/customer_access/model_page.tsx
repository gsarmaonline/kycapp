import { CustomerScopeKindsIndex } from '../customer_scope_kinds/customer_scope_kinds_index'
import { CustomerCapabilitiesIndex } from '../customer_capabilities/customer_capabilities_index'
import { CustomerMapPage } from '../customer_map/customer_map_page'
import { ConceptDocsLink } from '../../components/ConceptDocsLink'

/**
 * Model: what can exist here, and what can be done to it.
 *
 * Scope kinds, capabilities and the map were three sidebar items presented as
 * peers of Grants and Roles, and they are nothing like peers. They are the
 * vocabulary: declared once, early, and rarely touched again. Worse, the
 * prerequisite chain between them was invisible — a grant needs a declared
 * scope kind *and* a capability *and* a role, and you discovered that as a
 * validation error on the last page of four.
 *
 * Putting them on one page makes the chain visible in the place you would act
 * on it, and puts the map beside the declarations it draws, so editing a
 * capability visibly changes the picture.
 */
export function CustomerModelPage() {
  return (
    <div className="stack">
      <header className="page-intro">
        <h1>Model</h1>
        <p className="muted">
          What can exist in your product, and what can be done to it. Declare this first: a grant
          needs a scope kind and a capability before it can be written.{' '}
          <ConceptDocsLink slug="customer-access" label="How customer access works" />
        </p>
      </header>

      <section className="model-section">
        <CustomerScopeKindsIndex />
      </section>

      <section className="model-section">
        <CustomerCapabilitiesIndex />
      </section>

      {/* The map is a view, not an object — you cannot create one — so it sits
          under the declarations rather than beside them in the nav. */}
      <section className="model-section">
        <CustomerMapPage />
      </section>
    </div>
  )
}
