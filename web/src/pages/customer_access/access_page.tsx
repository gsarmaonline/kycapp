import { CustomerGrantsIndex } from '../customer_grants/customer_grants_index'
import { CustomerEdgesPage } from '../customer_edges/customer_edges_page'
import { ConceptDocsLink } from '../../components/ConceptDocsLink'

/**
 * Access: who has what, and why.
 *
 * Grants and Edges were two sidebar items that both gave access, and until
 * recently they wrote to two stores that could not see each other — a grant
 * issued here did not affect POST /check, and an edge written there never
 * appeared in GET /access. A merchant had no way to know which to use, and no
 * reason to suspect that using one made the other lie.
 *
 * They are one store now, so they are one page. The two halves that remain are
 * not two mechanisms but two authors: the composer a person uses to say "give
 * this person this role, here, until then", and the facts a backend writes as
 * its own resources are created.
 *
 * Those halves differ by orders of magnitude in volume — tens of hand-written
 * grants against potentially millions of machine-written edges — so the
 * composer comes first and the facts are a query rather than a listing.
 */
export function CustomerAccessPage() {
  return (
    <div className="stack">
      <header className="page-intro">
        <h1>Access</h1>
        <p className="muted">
          Who holds what, and why. Grants are what a person writes; edges are what your backend
          writes as resources appear. Both land in the same store, so both answer the same
          question.{' '}
          <ConceptDocsLink slug="customer-grants" label="How grants are evaluated" />
        </p>
      </header>

      <section className="model-section">
        <CustomerGrantsIndex />
      </section>

      <section className="model-section">
        <CustomerEdgesPage />
      </section>
    </div>
  )
}
