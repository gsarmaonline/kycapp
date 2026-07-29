import { Link, useParams } from 'react-router-dom'
import { docsBasePath } from '../../templates/paths'

export function DocsVariablesPage() {
  const { orgId } = useParams()

  return (
    <article className="docs-variables prose-docs">
      <p>
        One shared path vocabulary powers emails, outbound webhook templates, automation
        conditions, <code>db_insert</code> mappings, and trigger-parameter matching. See also the{' '}
        <Link to={docsBasePath(orgId)}>concepts</Link> and the{' '}
        <Link to={`${docsBasePath(orgId)}/api`}>API reference</Link>.
      </p>

      <h2>Syntax</h2>
      <table>
        <thead>
          <tr>
            <th>Surface</th>
            <th>Form</th>
            <th>Example</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Emails &amp; webhook JSON strings</td>
            <td>
              <code>{'{{path}}'}</code>
            </td>
            <td>
              <code>{'{{app_user.email}}'}</code>
            </td>
          </tr>
          <tr>
            <td>Conditions / <code>db_insert</code> mappings</td>
            <td>bare path</td>
            <td>
              <code>app_user.email</code>
            </td>
          </tr>
        </tbody>
      </table>
      <p>Braces are only for string templates. Paths are the same either way.</p>

      <h2>Canonical paths</h2>
      <table>
        <thead>
          <tr>
            <th>Path</th>
            <th>Meaning</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>
              <code>app_user.id</code>
            </td>
            <td>App user id</td>
          </tr>
          <tr>
            <td>
              <code>app_user.email</code>
            </td>
            <td>Email</td>
          </tr>
          <tr>
            <td>
              <code>app_user.display_name</code>
            </td>
            <td>Display name</td>
          </tr>
          <tr>
            <td>
              <code>app_user.status</code>
            </td>
            <td>Status</td>
          </tr>
          <tr>
            <td>
              <code>app_user.external_id</code>
            </td>
            <td>Merchant external id</td>
          </tr>
          <tr>
            <td>
              <code>app_user.&lt;attribute_key&gt;</code>
            </td>
            <td>
              Org attribute (e.g. <code>app_user.country</code>)
            </td>
          </tr>
          <tr>
            <td>
              <code>organisation.name</code>
            </td>
            <td>Organisation display name (email render context)</td>
          </tr>
          <tr>
            <td>
              <code>organisation_id</code> / <code>trigger</code>
            </td>
            <td>Run metadata</td>
          </tr>
        </tbody>
      </table>

      <h2>Library</h2>
      <ul>
        <li>
          <strong>Go:</strong> <code>core/automations</code> — <code>Lookup</code>,{' '}
          <code>RenderStringTemplate</code>, <code>RenderJSONTemplate</code>
        </li>
        <li>
          <strong>Web preview:</strong> <code>web/src/templates/paths.ts</code> — same path rules
        </li>
      </ul>
      <p>
        Email send uses <code>emailtemplates.Render</code> → <code>automations.RenderStringTemplate</code>.
      </p>

      <h2>Legacy aliases</h2>
      <p>
        <code>display_name</code>, <code>email</code>, <code>attributes.country</code>, and{' '}
        <code>org_name</code> still resolve; prefer canonical <code>app_user.*</code> /{' '}
        <code>organisation.name</code> paths.
      </p>

      <h2>Examples</h2>
      <pre className="preview-text">{`<p>Hi {{app_user.display_name}},</p>
<p>Welcome to {{organisation.name}}.</p>`}</pre>
      <pre className="preview-text">{`{
  "organisation_id": "{{organisation_id}}",
  "email": "{{app_user.email}}",
  "country": "{{app_user.country}}"
}`}</pre>
      <pre className="preview-text">{`{ "email": "app_user.email", "country": "app_user.country" }`}</pre>
    </article>
  )
}
