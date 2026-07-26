import type { ReactNode } from 'react'
import SwaggerUI from 'swagger-ui-react'
import 'swagger-ui-react/swagger-ui.css'

type DocsSwaggerProps = {
  hint: ReactNode
  specUrl: string
  rawLabel: string
}

export function DocsSwagger({ hint, specUrl, rawLabel }: DocsSwaggerProps) {
  return (
    <div className="docs-api">
      <p className="field-hint">
        {hint} Raw spec:{' '}
        <a href={specUrl} target="_blank" rel="noreferrer">
          {rawLabel}
        </a>
        .
      </p>
      <div className="docs-swagger">
        <SwaggerUI url={specUrl} docExpansion="list" defaultModelsExpandDepth={1} />
      </div>
    </div>
  )
}
