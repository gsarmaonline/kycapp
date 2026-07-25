import { Link, useParams } from 'react-router-dom'
import SwaggerUI from 'swagger-ui-react'
import 'swagger-ui-react/swagger-ui.css'
import { variablesDocsPath } from '../../templates/paths'

export function DocsApiPage() {
  const { orgId = '' } = useParams()

  return (
    <div className="docs-api">
      <p className="field-hint">
        Interactive OpenAPI 3 spec for all <code>/v1</code> endpoints (request/response shapes).
        Placeholder syntax for emails and webhooks is under{' '}
        <Link to={variablesDocsPath(orgId)}>Variables</Link>. Raw spec:{' '}
        <a href="/openapi.yaml" target="_blank" rel="noreferrer">
          openapi.yaml
        </a>
        .
      </p>
      <div className="docs-swagger">
        <SwaggerUI url="/openapi.yaml" docExpansion="list" defaultModelsExpandDepth={1} />
      </div>
    </div>
  )
}
