import { DOC_CONCEPTS, type DocConcept } from './docs_concepts'

export type DocSearchHit = {
  slug: string
  title: string
  summary: string
  /** The matching line, so a hit explains itself without being opened. */
  excerpt: string
}

/**
 * Everything about a concept that is worth matching against, flattened once.
 *
 * Body text and examples are included deliberately: someone searching "expires"
 * or "wildcard" is describing a problem, not naming a page, and the page titles
 * alone would never answer them.
 */
function haystack(c: DocConcept): string[] {
  const lines = [c.title, c.summary, ...c.body]
  for (const s of c.steps ?? []) lines.push(`${s.title}. ${s.detail}`)
  for (const e of c.examples ?? []) {
    lines.push(`${e.title}. ${e.problem}`)
    if (e.note) lines.push(e.note)
    // The grant blocks carry the concrete syntax, so someone searching
    // "expires" or "every capability" is searching for exactly these. Split by
    // line so an excerpt is one readable line rather than a whole block.
    lines.push(...e.grant.split('\n').filter(Boolean))
  }
  if (c.sample) {
    // The sample's code carries the API field names — all_capabilities,
    // constraint, scope_kind — which is precisely what someone reading the API
    // reference will type into this box.
    lines.push(c.sample.label, ...c.sample.code.split('\n').filter(Boolean))
  }
  return lines
}

/**
 * Matches concepts whose text contains every term, in any order and anywhere.
 *
 * All terms rather than any: with a corpus this small, "any" makes a second
 * word widen the results, which reads as the search ignoring what you typed.
 */
export function searchDocs(query: string): DocSearchHit[] {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (terms.length === 0) return []

  const hits: DocSearchHit[] = []
  for (const c of DOC_CONCEPTS) {
    const lines = haystack(c)
    const blob = lines.join(' \n ').toLowerCase()
    if (!terms.every((t) => blob.includes(t))) continue

    // Prefer a line carrying the first term over the summary, so the excerpt
    // shows why this matched rather than what the page is generally about.
    const line = lines.slice(1).find((l) => l.toLowerCase().includes(terms[0]))
    hits.push({ slug: c.slug, title: c.title, summary: c.summary, excerpt: line ?? c.summary })
  }

  // A title match is almost always what was wanted, so it sorts first.
  return hits.sort((a, b) => {
    const at = a.title.toLowerCase().includes(terms[0]) ? 0 : 1
    const bt = b.title.toLowerCase().includes(terms[0]) ? 0 : 1
    return at - bt
  })
}
