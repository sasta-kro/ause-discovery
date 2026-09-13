import { describe, expect, it } from 'vitest'
import type { SearchState } from './state'
import { applyFilterValue, buildSuggestionDefs, matchSuggestions, normalizeAlias, removeSuggestionRange, type SuggestionSource } from './suggestions'

const sources: SuggestionSource[] = [
  { stateField: 'academic_year', dimensionLabel: 'Academic year', cardinality: 'scalar', choices: [{ value: '2025', label: '2025' }, { value: '2024', label: '2024' }] },
  { stateField: 'semester', dimensionLabel: 'Semester', cardinality: 'scalar', choices: [{ value: 'first', label: 'First semester' }, { value: 'second', label: 'Second semester' }] },
  { stateField: 'program_key', dimensionLabel: 'Program', cardinality: 'multiple', choices: [{ value: 'information_technology', label: 'Information Technology' }] },
  { stateField: 'course_key', dimensionLabel: 'Course', cardinality: 'multiple', choices: [{ value: 'senior_project', label: 'Senior Project' }] },
  { stateField: 'category_key', dimensionLabel: 'Category', cardinality: 'multiple', choices: [{ value: 'game', label: 'Game' }, { value: 'go', label: 'Go' }] },
  { stateField: 'platform_key', dimensionLabel: 'Platform', cardinality: 'multiple', choices: [{ value: 'game', label: 'Game' }] },
  { stateField: 'domain_key', dimensionLabel: 'Domain', cardinality: 'multiple', choices: [{ value: 'machine_learning', label: 'Machine Learning' }] },
]

const defs = buildSuggestionDefs(sources)
const emptyState: SearchState = { limit: 20 }

function matches(text: string, applied: SearchState = emptyState) {
  return matchSuggestions(defs, { text, focused: true, collapsedCaret: true, caretAtEnd: true }, applied)
}

describe('filter suggestion matching', () => {
  it('normalizes case, underscores, and whitespace', () => {
    expect(normalizeAlias(' Machine  Learning ')).toBe('machine learning')
    expect(normalizeAlias('information_technology')).toBe('information technology')
  })

  it('matches exact and prefix case-insensitively', () => {
    expect(matches('GAM').map((s) => s.id)).toContain('category_key:game')
    expect(matches('Best GAM').map((s) => s.id)).toContain('category_key:game')
    expect(matches('SECOND SEME').map((s) => s.id)).toContain('semester:second')
  })

  it('never matches fuzzily', () => {
    expect(matches('fuck')).toEqual([])
    expect(matches('gmae')).toEqual([])
    expect(matches('machin lerning')).toEqual([])
  })

  it('requires three characters or a complete shorter alias', () => {
    expect(matches('ga').map((s) => s.id)).toEqual([])
    expect(matches('go').map((s) => s.id)).toContain('category_key:go')
  })

  it('matches only existing four-digit academic years exactly', () => {
    expect(matches('2025').map((s) => s.id)).toEqual(['academic_year:2025'])
    expect(matches('202')).toEqual([])
    expect(matches('2026')).toEqual([])
  })

  it('suppresses on trailing whitespace, caret away from the end, selection, or blur', () => {
    expect(matches('gam ')).toEqual([])
    expect(matchSuggestions(defs, { text: 'best gam', focused: true, collapsedCaret: true, caretAtEnd: false }, emptyState)).toEqual([])
    expect(matchSuggestions(defs, { text: 'best gam', focused: true, collapsedCaret: false, caretAtEnd: true }, emptyState)).toEqual([])
    expect(matchSuggestions(defs, { text: 'best gam', focused: false, collapsedCaret: true, caretAtEnd: true }, emptyState)).toEqual([])
  })

  it('selects the longest eligible suffix and consumes only that range', () => {
    const suggestion = matches('best gam').find((s) => s.id === 'category_key:game')!
    expect(suggestion.start).toBe(5)
    expect(removeSuggestionRange('best gam', suggestion.start, suggestion.end)).toBe('best')
  })

  it('does not match a value that is not the nearest suffix', () => {
    expect(matches('best game project').map((s) => s.id)).not.toContain('category_key:game')
    expect(matches('game project')).toEqual([])
  })

  it('matches only the year suffix first in a multiword query', () => {
    expect(matches('machine learning 2025').map((s) => s.id)).toEqual(['academic_year:2025'])
  })

  it('matches a multiword controlled value only when it is a real controlled value', () => {
    expect(matches('machine learning').map((s) => s.id)).toContain('domain_key:machine_learning')
    expect(matches('machine learning 2025').map((s) => s.id)).not.toContain('domain_key:machine_learning')
  })

  it('ranks exact matches before prefix matches', () => {
    const ids = matches('game').map((s) => s.id)
    expect(ids[0]).toBe('category_key:game')
    // Both Game collisions remain visible with dimension labels.
    expect(ids).toContain('platform_key:game')
  })

  it('orders collisions deterministically by dimension priority', () => {
    const list = matches('game')
    expect(list.map((s) => s.dimensionLabel)).toEqual(['Category', 'Platform'])
  })

  it('excludes already-selected values', () => {
    expect(matches('game', { limit: 20, category_key: ['game'] }).map((s) => s.id)).toEqual(['platform_key:game'])
    expect(matches('2025', { limit: 20, academic_year: 2025 })).toEqual([])
    expect(matches('first', { limit: 20, semester: 'first' })).toEqual([])
  })

  it('bounds the suggestion list', () => {
    const many: SuggestionSource[] = [{ stateField: 'category_key', dimensionLabel: 'Category', cardinality: 'multiple', choices: Array.from({ length: 10 }, (_, i) => ({ value: `term${i}`, label: `Term${i}` })) }]
    const manyDefs = buildSuggestionDefs(many)
    expect(matchSuggestions(manyDefs, { text: 'term', focused: true, collapsedCaret: true, caretAtEnd: true }, emptyState)).toHaveLength(6)
  })

  it('preserves unmatched text when removing the consumed range', () => {
    expect(removeSuggestionRange('machine learning 2025', 17, 26)).toBe('machine learning')
    expect(removeSuggestionRange('machine learning  2025', 18, 27)).toBe('machine learning')
    expect(removeSuggestionRange('2025', 0, 4)).toBe('')
  })
})

describe('shared filter application', () => {
  it('replaces scalar values and appends multiple values deterministically', () => {
    const replaced = applyFilterValue({ limit: 20, academic_year: 2024 }, 'academic_year', '2025')
    expect(replaced.academic_year).toBe(2025)
    const appended = applyFilterValue({ limit: 20, category_key: ['game'] }, 'category_key', 'web')
    expect(appended.category_key).toEqual(['game', 'web'])
    expect(applyFilterValue(appended, 'category_key', 'game').category_key).toEqual(['game', 'web'])
    expect(applyFilterValue({ limit: 20 }, 'semester', 'summer').semester).toBe('summer')
  })
})
