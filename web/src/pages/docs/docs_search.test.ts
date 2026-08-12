import { describe, expect, it } from 'vitest'
import { searchDocs } from './docs_search'

describe('searchDocs', () => {
  it('returns nothing for an empty query', () => {
    expect(searchDocs('')).toEqual([])
    expect(searchDocs('   ')).toEqual([])
  })

  it('finds a concept by its title', () => {
    const hits = searchDocs('grants')
    expect(hits.map((h) => h.slug)).toContain('customer-grants')
  })

  // The point of searching body text: people search for the problem they have,
  // not the name of the page that solves it.
  it('finds a concept by words that appear only in its body', () => {
    const hits = searchDocs('offboard')
    expect(hits.map((h) => h.slug)).toContain('customer-access-examples')
  })

  it('requires every term, so a second word narrows rather than widens', () => {
    const one = searchDocs('capability')
    const two = searchDocs('capability wildcard')
    expect(two.length).toBeLessThanOrEqual(one.length)
    expect(searchDocs('grants zzzznotaword')).toEqual([])
  })

  // Someone reading the API reference searches for the field name, not for the
  // prose that describes it.
  it('finds API field names that appear only in a code sample', () => {
    expect(searchDocs('except_scopes').map((h) => h.slug)).toContain('customer-access')
    expect(searchDocs('all_capabilities').length).toBeGreaterThan(0)
  })

  it('is case insensitive', () => {
    expect(searchDocs('ROLES').length).toEqual(searchDocs('roles').length)
  })

  it('sorts title matches first', () => {
    const hits = searchDocs('roles')
    expect(hits[0].slug).toBe('customer-roles')
  })

  // An excerpt that just repeated the summary would not say why this matched.
  it('excerpts a line containing the term', () => {
    const hit = searchDocs('expires')[0]
    expect(hit.excerpt.toLowerCase()).toContain('expire')
  })
})
