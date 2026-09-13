import { describe, expect, it } from 'vitest'
import { displayedSort, parseSearchState, resetSearchCursor, serializeSearchState } from './state'

describe('URL-driven search state', () => {
  it('parses all supported repeated and scalar facet values', () => {
    const state = parseSearchState(new URLSearchParams('q=vision&academic_year=2024&semester=first&program_key=it&program_key=business&student_id=1234567&major_key=software_engineering&topic_key=ai&has_report=true&sort=newest&cursor=next'))
    expect(state).toMatchObject({ q: 'vision', academic_year: 2024, semester: 'first', program_key: ['it', 'business'], student_id: '1234567', has_report: true, sort: 'newest', cursor: 'next' })
    expect(state.major_key).toBeUndefined()
    expect(state.topic_key).toBeUndefined()
    const serialized = serializeSearchState(resetSearchCursor(state)).toString()
    expect(serialized).not.toContain('cursor=')
    expect(serialized).not.toContain('major_key=')
    expect(serialized).not.toContain('topic_key=')
  })

  it('normalizes invalid values without retaining unsafe state', () => {
    const state = parseSearchState(new URLSearchParams('academic_year=unknown&student_id=12&sort=unexpected&limit=1000'))
    expect(state.academic_year).toBeUndefined()
    expect(state.student_id).toBeUndefined()
    expect(state.sort).toBeUndefined()
    expect(state.limit).toBe(20)
  })

  it('keeps an omitted sort implicit and displays it contextually', () => {
    const empty = parseSearchState(new URLSearchParams(''))
    expect(empty.sort).toBeUndefined()
    expect(displayedSort(empty)).toBe('newest')
    expect(serializeSearchState(empty).toString()).not.toContain('sort=')

    const textual = parseSearchState(new URLSearchParams('q=vision'))
    expect(textual.sort).toBeUndefined()
    expect(displayedSort(textual)).toBe('relevance')
    expect(serializeSearchState(textual).toString()).not.toContain('sort=')

    // Whitespace-only text is an empty search.
    const blank = parseSearchState(new URLSearchParams('q=%20%20'))
    expect(displayedSort(blank)).toBe('newest')
  })

  it('serializes every explicit sort selection including relevance', () => {
    for (const sort of ['relevance', 'newest', 'oldest', 'title'] as const) {
      const state = parseSearchState(new URLSearchParams(`q=vision&sort=${sort}`))
      expect(state.sort).toBe(sort)
      expect(displayedSort(state)).toBe(sort)
      expect(serializeSearchState(state).toString()).toContain(`sort=${sort}`)
    }
  })

  it('keeps an explicit sort authoritative regardless of query state', () => {
    expect(displayedSort(parseSearchState(new URLSearchParams('sort=newest')))).toBe('newest')
    expect(displayedSort(parseSearchState(new URLSearchParams('q=vision&sort=title')))).toBe('title')
  })
})
