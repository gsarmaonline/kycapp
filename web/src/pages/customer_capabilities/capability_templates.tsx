import { useEffect, useState } from 'react'
import { applyCapabilityTemplate, listCapabilityTemplates, type CapabilityTemplate } from '../../api'

/**
 * Starter sets, offered on an empty vocabulary.
 *
 * Nothing is seeded into customer access on signup, and that is deliberate:
 * this vocabulary is the merchant's own, so a guessed default is a claim about
 * a product KYC has never seen. Merchants work around a wrong default rather
 * than delete it, and then it has to be supported for ever.
 *
 * The cold start is still real. A grant needs a scope kind, a capability and a
 * role before it can be written, and all three pages open empty. So a template
 * is offered rather than applied: the whole list is visible before anything is
 * written, one click applies it, and every row it creates is marked as
 * template-sourced so the provenance outlives whoever clicked.
 *
 * Shown only while the list is empty. Once a merchant has a vocabulary, this is
 * someone else's idea of their product.
 */
export function CapabilityTemplates({
  orgId,
  onApplied,
}: {
  orgId: string
  onApplied: () => void
}) {
  const [templates, setTemplates] = useState<CapabilityTemplate[]>([])
  const [open, setOpen] = useState<string | null>(null)
  const [applying, setApplying] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    void listCapabilityTemplates(orgId)
      .then((r) => live && setTemplates(r.items))
      .catch(() => live && setTemplates([]))
    return () => {
      live = false
    }
  }, [orgId])

  if (templates.length === 0) return null

  async function apply(key: string) {
    setApplying(key)
    setError(null)
    try {
      await applyCapabilityTemplate(orgId, key)
      onApplied()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not apply the template')
    } finally {
      setApplying(null)
    }
  }

  return (
    <section className="cap-templates">
      <h3 className="cap-templates-title">Start from a template</h3>
      <p className="cap-templates-hint">
        A starting point, not a default. Nothing is applied until you choose one,
        and everything it creates is yours to edit or delete afterwards.
      </p>
      {error && <p className="error">{error}</p>}
      <div className="cap-template-list">
        {templates.map((t) => (
          <article key={t.key} className="cap-template">
            <div className="cap-template-head">
              <div>
                <strong>{t.name}</strong>
                <p className="cap-template-desc">{t.description}</p>
              </div>
              <button
                type="button"
                className="button"
                disabled={applying !== null}
                onClick={() => void apply(t.key)}
              >
                {applying === t.key
                  ? 'Applying…'
                  : `Apply ${t.items.length}${t.roles?.length ? ` + ${t.roles.length} roles` : ''}`}
              </button>
            </div>
            <button
              type="button"
              className="link-btn cap-template-toggle"
              aria-expanded={open === t.key}
              onClick={() => setOpen(open === t.key ? null : t.key)}
            >
              {open === t.key ? 'Hide' : 'Show'} what this declares
            </button>
            {open === t.key && (
              <>
                <ul className="cap-template-items">
                  {t.items.map((item) => (
                    <li key={item.key}>
                      <code>{item.key}</code>
                      <span>{item.description}</span>
                    </li>
                  ))}
                </ul>
                {t.roles && t.roles.length > 0 && (
                  <ul className="cap-template-items">
                    {t.roles.map((role) => (
                      <li key={role.key}>
                        <code>{role.key}</code>
                        <span>
                          {role.extends && role.extends.length > 0
                            ? `extends ${role.extends.join(', ')}, plus `
                            : ''}
                          {role.capabilities.join(', ')}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
                <p className="cap-template-desc">
                  No grants. A role gives nobody anything until you issue one.
                </p>
              </>
            )}
          </article>
        ))}
      </div>
    </section>
  )
}
