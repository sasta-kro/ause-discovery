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
    expect(applyFilterValue({ limit: 20 }, 'person_id', '018f-phyo').person_id).toEqual(['018f-phyo'])
    expect(applyFilterValue({ limit: 20, advisor_id: ['018f-other'] }, 'advisor_id', '018f-phyo').advisor_id).toEqual(['018f-other', '018f-phyo'])
  })
})

const personID = '018f0000-0000-7000-8000-00000000phyo'
const personSources: SuggestionSource[] = [
  { stateField: 'person_id', dimensionLabel: 'People', cardinality: 'multiple', choices: [{ value: personID, label: 'Phyo Min Tun' }, { value: '018f0000-0000-7000-8000-00000000thiri', label: 'Thiri Su' }] },
  { stateField: 'advisor_id', dimensionLabel: 'Advisor', cardinality: 'multiple', choices: [{ value: personID, label: 'Phyo Min Tun' }] },
]
const personDefs = buildSuggestionDefs(personSources)

function personMatches(text: string, applied: SearchState = emptyState) {
  return matchSuggestions(personDefs, { text, focused: true, collapsedCaret: true, caretAtEnd: true }, applied)
}

describe('person-name suggestion matching', () => {
  it('derives display-name aliases only, never UUID aliases', () => {
    const phyos = personDefs.filter((def) => def.value === personID)
    expect(phyos).toHaveLength(2)
    for (const def of phyos) expect(def.aliases).toEqual(['phyo min tun'])
    expect(personMatches('018f')).toEqual([])
    expect(personMatches('018f0000')).toEqual([])
  })

  it('matches case-insensitive name prefixes including the exact full name', () => {
    for (const text of ['Phyo', 'phyo', 'PhYo Mi', 'Phyo Min', 'Phyo Min Tun']) {
      expect(personMatches(text).map((s) => s.id)).toContain(`person_id:${personID}`)
    }
  })

  it('never matches reordered segments, truncations, or misspellings', () => {
    expect(personMatches('Min Tun')).toEqual([])
    expect(personMatches('Pyo')).toEqual([])
    expect(personMatches('Phyo Tin')).toEqual([])
    expect(personMatches('Phyo Min Tum')).toEqual([])
    expect(personMatches('Ph')).toEqual([])
  })

  it('keeps a matched Person prefix eligible across a trailing space', () => {
    expect(personMatches('Phyo ').map((s) => s.id)).toContain(`person_id:${personID}`)
    expect(personMatches('Phyo Min ').map((s) => s.id)).toContain(`person_id:${personID}`)
    expect(personMatches('Phyo Min Tun ').map((s) => s.id)).toEqual([`person_id:${personID}`, `advisor_id:${personID}`])
    // A space after an incomplete name segment is not a segment boundary.
    expect(personMatches('Phyo Mi ')).toEqual([])
    const suggestion = personMatches('archive Phyo ').find((s) => s.id === `person_id:${personID}`)!
    expect(removeSuggestionRange('archive Phyo ', suggestion.start, suggestion.end)).toBe('archive')
  })

  it('keeps standard trailing-whitespace suppression for other dimensions', () => {
    expect(personMatches('unmatched ')).toEqual([])
    expect(personMatches('Thiri ').map((s) => s.id)).toEqual(['person_id:018f0000-0000-7000-8000-00000000thiri'])
    expect(matches('gam ')).toEqual([])
    expect(matches('2025 ')).toEqual([])
  })

  it('consumes only the nearest Person suffix and preserves earlier free text', () => {
    const suggestion = personMatches('archive Phyo Mi').find((s) => s.id === `person_id:${personID}`)!
    expect(suggestion.start).toBe(8)
    expect(suggestion.end).toBe(15)
    expect(removeSuggestionRange('archive Phyo Mi', suggestion.start, suggestion.end)).toBe('archive')
  })

  it('does not scan backward past an unrelated trailing term', () => {
    expect(personMatches('archive Phyo Mi report')).toEqual([])
    expect(personMatches('archive Phyo Mi report ')).toEqual([])
  })

  it('shows one Person in both dimensions as two deterministic labelled rows', () => {
    const list = personMatches('Phyo Min Tun')
    expect(list.map((s) => s.dimensionLabel)).toEqual(['People', 'Advisor'])
    expect(list.map((s) => s.valueLabel)).toEqual(['Phyo Min Tun', 'Phyo Min Tun'])
  })

  it('excludes selected values per dimension only', () => {
    const bothSelected: SearchState = { limit: 20, person_id: [personID], advisor_id: [personID] }
    expect(personMatches('Phyo Min Tun', { limit: 20, person_id: [personID] }).map((s) => s.id)).toEqual([`advisor_id:${personID}`])
    expect(personMatches('Phyo Min Tun', { limit: 20, advisor_id: [personID] }).map((s) => s.id)).toEqual([`person_id:${personID}`])
    expect(personMatches('Phyo Min Tun', bothSelected)).toEqual([])
  })

  it('keeps the six-result bound with person names', () => {
    const many: SuggestionSource[] = [{ stateField: 'person_id', dimensionLabel: 'People', cardinality: 'multiple', choices: Array.from({ length: 10 }, (_, i) => ({ value: `018f0000-0000-7000-8000-${String(i).padStart(12, '0')}`, label: `Person Number ${i}` })) }]
    const manyDefs = buildSuggestionDefs(many)
    expect(matchSuggestions(manyDefs, { text: 'person number', focused: true, collapsedCaret: true, caretAtEnd: true }, emptyState)).toHaveLength(6)
  })
})
