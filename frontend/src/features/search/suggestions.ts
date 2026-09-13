import type { SearchState } from './state'

// Search Filter Suggestions: conservative local conversion of one
// recognized trailing query fragment into one existing structured filter.
// Matching is exact or prefix, case-insensitive, never fuzzy.

export type SuggestionField = 'academic_year' | 'semester' | 'program_key' | 'course_key' | 'category_key' | 'platform_key' | 'domain_key' | 'technology_key'

export type FilterCardinality = 'scalar' | 'multiple'

// SuggestionDef is one controlled value eligible for suggestion. Ranges are
// attached per input by matchSuggestions.
export type SuggestionDef = {
  id: string
  stateField: SuggestionField
  dimensionLabel: string
  value: string
  valueLabel: string
  cardinality: FilterCardinality
  priority: number
  aliases: string[]
}

// Suggestion is one ranked match against the current input, carrying the
// exact source range acceptance will remove.
export type Suggestion = SuggestionDef & {
  start: number
  end: number
  exact: boolean
  consumed: number
}

export type SuggestionSource = {
  stateField: SuggestionField
  dimensionLabel: string
  cardinality: FilterCardinality
  choices: Array<{ value: string; label: string }>
}

const maxSuggestions = 6
const maxSuffixLength = 80

// normalizeAlias lowercases, turns underscores into spaces, and collapses
// surrounding and repeated whitespace so stable keys match their labels.
export function normalizeAlias(alias: string): string {
  return alias.replaceAll('_', ' ').toLowerCase().replace(/\s+/g, ' ').trim()
}

// buildSuggestionDefs derives one definition per controlled value, in the
// stable panel dimension order the caller provides. The visible label and
// the underscore-spaced key are both aliases; nothing else is invented.
export function buildSuggestionDefs(sources: SuggestionSource[]): SuggestionDef[] {
  const defs: SuggestionDef[] = []
  for (const [priority, source] of sources.entries()) {
    for (const choice of source.choices) {
      const aliases = [choice.label, choice.value.replaceAll('_', ' ')]
        .map((alias) => normalizeAlias(alias))
        .filter((alias, index, all) => alias !== '' && all.indexOf(alias) === index)
      defs.push({
        id: `${source.stateField}:${choice.value}`,
        stateField: source.stateField,
        dimensionLabel: source.dimensionLabel,
        value: choice.value,
        valueLabel: choice.label,
        cardinality: source.cardinality,
        priority,
        aliases,
      })
    }
  }
  return defs
}

export type SuggestionInputState = {
  text: string
  focused: boolean
  collapsedCaret: boolean
  caretAtEnd: boolean
}

// isSuggestionEligible applies the caret boundary: focus, a collapsed caret
// at the end, and no trailing whitespace.
export function isSuggestionEligible(input: SuggestionInputState): boolean {
  return input.focused && input.collapsedCaret && input.caretAtEnd && !/\s$/.test(input.text)
}

// candidateSuffixes lists suffixes that end at the caret (here the text end)
// and begin at the start of the input or a word boundary, bounded in length.
export function candidateSuffixes(text: string): Array<{ start: number; end: number; normalized: string }> {
  const suffixes: Array<{ start: number; end: number; normalized: string }> = []
  const start = Math.max(0, text.length - maxSuffixLength)
  for (let index = start; index < text.length; index += 1) {
    const beginsInput = index === 0
    const beginsBoundary = index > 0 && /\s/.test(text[index - 1])
    if (!beginsInput && !beginsBoundary) continue
    const fragment = text.slice(index)
    const normalized = normalizeAlias(fragment)
    if (normalized !== '') suffixes.push({ start: index, end: text.length, normalized })
  }
  return suffixes
}

function fragmentEligible(normalized: string, def: SuggestionDef): boolean {
  // Academic years need exactly four digits that fully match an alias.
  if (def.stateField === 'academic_year') {
    return /^\d{4}$/.test(normalized) && def.aliases.some((alias) => alias === normalized)
  }
  if (normalized.length >= 3) return true
  // A shorter fragment is eligible only as a complete alias, such as Go.
  return def.aliases.some((alias) => alias === normalized)
}

// matchSuggestions returns the ranked bounded suggestion list for the input.
// Each alias keeps its longest eligible matching suffix; ranking is exact
// before prefix, longer consumed suffixes first, then dimension priority
// (the caller-provided panel order), then label, then id.
export function matchSuggestions(defs: SuggestionDef[], input: SuggestionInputState, applied: SearchState): Suggestion[] {
  if (!isSuggestionEligible(input)) return []
  const suffixes = candidateSuffixes(input.text)
  const matches: Suggestion[] = []
  for (const def of defs) {
    if (alreadyApplied(def, applied)) continue
    let best: Suggestion | undefined
    for (const suffix of suffixes) {
      for (const alias of def.aliases) {
        const exact = alias === suffix.normalized
        const prefix = alias.startsWith(suffix.normalized)
        if (!exact && !prefix) continue
        if (!fragmentEligible(suffix.normalized, def)) continue
        const candidate: Suggestion = {
          ...def,
          start: suffix.start,
          end: suffix.end,
          exact,
          consumed: suffix.normalized.length,
        }
        if (!best || better(candidate, best)) best = candidate
      }
    }
    if (best) matches.push(best)
  }
  return matches.sort(compareSuggestions).slice(0, maxSuggestions)
}

function better(candidate: Suggestion, current: Suggestion): boolean {
  return compareSuggestions(candidate, current) < 0
}

function compareSuggestions(left: Suggestion, right: Suggestion): number {
  if (left.exact !== right.exact) return left.exact ? -1 : 1
  if (left.consumed !== right.consumed) return right.consumed - left.consumed
  if (left.priority !== right.priority) return left.priority - right.priority
  if (left.dimensionLabel !== right.dimensionLabel) return left.dimensionLabel.localeCompare(right.dimensionLabel)
  if (left.valueLabel !== right.valueLabel) return left.valueLabel.localeCompare(right.valueLabel)
  return left.id.localeCompare(right.id)
}

function alreadyApplied(def: SuggestionDef, applied: SearchState): boolean {
  if (def.stateField === 'academic_year') return applied.academic_year === Number(def.value)
  if (def.stateField === 'semester') return applied.semester === def.value
  const selected = applied[def.stateField]
  return Array.isArray(selected) && selected.includes(def.value)
}

// removeSuggestionRange removes exactly the consumed source range and the
// boundary whitespace that would otherwise double or trail, preserving every
// other character and the word order.
export function removeSuggestionRange(text: string, start: number, end: number): string {
  const before = text.slice(0, start).replace(/\s+$/, '')
  return before + text.slice(end).replace(/^\s+/, '')
}

const suggestionFields: ReadonlySet<string> = new Set<SuggestionField>(['academic_year', 'semester', 'program_key', 'course_key', 'category_key', 'platform_key', 'domain_key', 'technology_key'])

// isSuggestionField narrows an array filter key to the dimensions the left
// panel and suggestions share.
export function isSuggestionField(key: string): key is SuggestionField {
  return suggestionFields.has(key)
}

// applyFilterValue is the one shared filter-application operation used by
// both the left filter panel and accepted suggestions. Scalar dimensions
// replace their value; multiple dimensions append a missing value in place.
export function applyFilterValue(state: SearchState, field: SuggestionField, value: string): SearchState {
  if (field === 'academic_year') {
    const year = Number(value)
    return Number.isInteger(year) ? { ...state, academic_year: year } : state
  }
  if (field === 'semester') {
    return value === 'first' || value === 'second' || value === 'summer' ? { ...state, semester: value } : state
  }
  const selected = state[field]
  if (Array.isArray(selected) && selected.includes(value)) return state
  return { ...state, [field]: [...(selected ?? []), value] }
}
