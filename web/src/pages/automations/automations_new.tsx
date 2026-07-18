import { useNavigate, useParams } from 'react-router-dom'
import { createAutomation } from '../../api'
import { PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'
import { AutomationsForm } from './automations_form'

export function AutomationsNew() {
  const { orgId = '' } = useParams()
  const navigate = useNavigate()

  return (
    <section>
      <PageHeader title="Create automation" />
      <AutomationsForm
        submitLabel="Create"
        cancelTo={resourcePath(orgId, 'automations')}
        onSubmit={async (input) => {
          const a = await createAutomation(orgId, input)
          navigate(resourcePath(orgId, 'automations', a.id))
        }}
      />
    </section>
  )
}
