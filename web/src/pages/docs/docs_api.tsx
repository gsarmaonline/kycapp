import { Link, useParams } from 'react-router-dom'
import { orgPath } from '../../org_nav'
import { variablesDocsPath } from '../../templates/paths'
import { DocsSwagger } from './docs_swagger'

/** Merchant Integration API — org API key surface. */
export function DocsIntegrationApiPage() {
  const { orgId = '' } = useParams()

  return (
    <DocsSwagger
      specUrl="/openapi-integration.yaml"
      rawLabel="openapi-integration.yaml"
      hint={
        <>
          APIs for merchant backends calling KYC with an{' '}
          <Link to={orgPath(orgId, 'api-keys')}>organisation API key</Link> (
          <code>Authorization: Bearer kyc_…</code>): app users, attributes, product plans, and
          entitlement checks. Placeholder syntax is under{' '}
          <Link to={variablesDocsPath(orgId)}>Variables</Link>. Operator UI routes (OAuth, members,
          permissions) are under <Link to={`${orgPath(orgId, 'docs')}/operator`}>Operator API</Link>.
        </>
      }
    />
  )
}

/** Full operator / platform OpenAPI. */
export function DocsOperatorApiPage() {
  const { orgId = '' } = useParams()

  return (
    <DocsSwagger
      specUrl="/openapi.yaml"
      rawLabel="openapi.yaml"
      hint={
        <>
          Full OpenAPI for the KYC operator UI and platform ops (sessions, members, roles,
          permissions, settings, automations, billing admin). For merchant backends, prefer the{' '}
          <Link to={orgPath(orgId, 'docs')}>Integration API</Link>.
        </>
      }
    />
  )
}

/** @deprecated use DocsIntegrationApiPage */
export const DocsApiPage = DocsIntegrationApiPage
