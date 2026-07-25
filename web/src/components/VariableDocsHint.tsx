import type { ReactNode } from 'react'
import { Link, useParams } from 'react-router-dom'
import { variablesDocsPath } from '../templates/paths'

/** Inline pointer to the shared variable referencing documentation. */
export function VariableDocsHint({ children }: { children?: ReactNode }) {
  const { orgId = '' } = useParams()
  return (
    <span className="field-hint variable-docs-hint">
      {children ?? (
        <>
          Uses the shared <code>{'{{path}}'}</code> / <code>app_user.*</code> vocabulary.
        </>
      )}{' '}
      <Link to={variablesDocsPath(orgId)}>Variable referencing guide</Link>
    </span>
  )
}
