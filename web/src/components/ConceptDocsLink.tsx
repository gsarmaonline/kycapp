import { Link, useParams } from 'react-router-dom'
import { docsBasePath } from '../templates/paths'

/**
 * Inline pointer from a resource page to the concept that explains it. The
 * page says what the object is; the concept says why it exists and where it
 * sits in the flow, which is too much to repeat above every table.
 */
export function ConceptDocsLink({ slug, label }: { slug: string; label: string }) {
  const { orgId = '' } = useParams()
  return (
    <Link className="concept-docs-link" to={`${docsBasePath(orgId)}/concepts/${slug}`}>
      {label}
    </Link>
  )
}
