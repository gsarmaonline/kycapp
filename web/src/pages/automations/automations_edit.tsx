import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { getAutomation, updateAutomation, type Automation } from '../../api'
import { PageHeader } from '../../crud/ui'
import { resourcePath } from '../../org_nav'
import { AutomationsForm } from './automations_form'

export function AutomationsEdit() {
  const { orgId = '', id = '' } = useParams()
  const navigate = useNavigate()
  const [item, setItem] = useState<Automation | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void getAutomation(id)
      .then(setItem)
      .catch((e) => setError(e instanceof Error ? e.message : 'Not found'))
  }, [id])

  if (error) return <p className="error">{error}</p>
  if (!item) return <p>Loading…</p>

  return (
    <section>
      <PageHeader title="Edit automation" />
      <AutomationsForm
        submitLabel="Save"
        cancelTo={resourcePath(orgId, 'automations', id)}
        initial={{
          name: item.name,
          trigger: item.trigger,
          enabled: item.enabled,
          conditions: item.conditions?.all ?? [],
          actions: item.actions ?? [],
        }}
        onSubmit={async (input) => {
          await updateAutomation(id, input)
          navigate(resourcePath(orgId, 'automations', id))
        }}
      />
    </section>
  )
}
