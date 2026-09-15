import type { FilterChoice } from './filter-controls'

// Filter panel option ordering. One pure non-mutating module serves the
// repeated left-panel presentation ordering; Search Filter Suggestions keep
// their own ranking and never use these comparators.

export type OptionOrderMode = 'occurrence' | 'alphabetical'

// compareByLabel orders by normalized visible label ascending, then by the
// stable choice value, using a predictable case-insensitive comparison.
function compareByLabel(left: FilterChoice, right: FilterChoice): number {
  const leftLabel = left.label.toLocaleLowerCase()
  const rightLabel = right.label.toLocaleLowerCase()
  if (leftLabel !== rightLabel) return leftLabel < rightLabel ? -1 : 1
  if (left.value !== right.value) return left.value < right.value ? -1 : 1
  return 0
}

function counted(choice: FilterChoice): boolean {
  return typeof choice.count === 'number' && choice.count > 0
}

// orderYearChoicesDescending sorts Academic year choices numerically newest
// first. Unexpected nonnumeric values follow valid years deterministically.
export function orderYearChoicesDescending(choices: FilterChoice[]): FilterChoice[] {
  return [...choices].sort((left, right) => {
    const leftYear = yearOf(left)
    const rightYear = yearOf(right)
    if (leftYear !== undefined && rightYear !== undefined) return rightYear - leftYear
    if (leftYear !== undefined) return -1
    if (rightYear !== undefined) return 1
    return compareByLabel(left, right)
  })
}

function yearOf(choice: FilterChoice): number | undefined {
  return /^\d{4}$/.test(choice.value) ? Number(choice.value) : undefined
}

// orderChoicesByOccurrence sorts positive counts descending, keeps zero and
// missing counts in one trailing tier, and breaks ties by label then value.
export function orderChoicesByOccurrence(choices: FilterChoice[]): FilterChoice[] {
  return [...choices].sort((left, right) => {
    if (counted(left) !== counted(right)) return counted(left) ? -1 : 1
    if (counted(left) && counted(right)) {
      const difference = (right.count ?? 0) - (left.count ?? 0)
      if (difference !== 0) return difference
    }
    return compareByLabel(left, right)
  })
}

// orderChoicesAlphabetically ignores counts and sorts by label then value.
export function orderChoicesAlphabetically(choices: FilterChoice[]): FilterChoice[] {
  return [...choices].sort(compareByLabel)
}
