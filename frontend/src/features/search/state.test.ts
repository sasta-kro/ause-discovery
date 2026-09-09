import { describe, expect, it } from 'vitest'
import { parseSearchState, resetSearchCursor, serializeSearchState } from './state'

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
    expect(state.sort).toBe('relevance')
    expect(state.limit).toBe(20)
  })
})
