import { describe, expect, it } from 'vitest'
import type { FilterChoice } from './filter-controls'
import { orderChoicesAlphabetically, orderChoicesByOccurrence, orderYearChoicesDescending } from './filter-ordering'

function choice(value: string, label: string, count?: number): FilterChoice {
  return { value, label, count }
}

describe('filter option ordering', () => {
  it('sorts academic years numerically newest first from mixed input', () => {
    const years = [choice('2021', '2021'), choice('2025', '2025'), choice('2016', '2016'), choice('2023', '2023')]
    expect(orderYearChoicesDescending(years).map((option) => option.value)).toEqual(['2025', '2023', '2021', '2016'])
  })

  it('places unexpected nonnumeric year values after valid years deterministically', () => {
    const years = [choice('unknown-b', 'Unknown B'), choice('2024', '2024'), choice('unknown-a', 'Unknown A'), choice('2026', '2026')]
    expect(orderYearChoicesDescending(years).map((option) => option.value)).toEqual(['2026', '2024', 'unknown-a', 'unknown-b'])
  })

  it('sorts positive counts descending', () => {
    const choices = [choice('react', 'React', 56), choice('nodejs', 'Node.js', 41), choice('go', 'Go', 1)]
    expect(orderChoicesByOccurrence([...choices].reverse()).map((option) => option.value)).toEqual(['react', 'nodejs', 'go'])
  })

  it('breaks equal positive counts by label then stable value', () => {
    const choices = [choice('zeta', 'Zeta', 5), choice('beta-2', 'Beta', 5), choice('beta-1', 'beta', 5), choice('alpha', 'Alpha', 5)]
    expect(orderChoicesByOccurrence(choices).map((option) => option.value)).toEqual(['alpha', 'beta-1', 'beta-2', 'zeta'])
  })

  it('keeps zero and missing counts after positive counts', () => {
    const choices = [choice('kotlin', 'Kotlin', 0), choice('uncounted', 'Uncounted'), choice('go', 'Go', 1), choice('react', 'React', 56)]
    expect(orderChoicesByOccurrence(choices).map((option) => option.value)).toEqual(['react', 'go', 'kotlin', 'uncounted'])
  })

  it('breaks zero and missing ties by label then stable value', () => {
    const choices = [choice('none-b', 'None B', 0), choice('none-a', 'None A', 0), choice('missing-z', 'Missing Z'), choice('missing-a', 'Missing A')]
    expect(orderChoicesByOccurrence(choices).map((option) => option.value)).toEqual(['missing-a', 'missing-z', 'none-a', 'none-b'])
  })

  it('orders alphabetically by label then value while ignoring counts', () => {
    const choices = [choice('zebra', 'Zebra', 99), choice('apple', 'apple', 0), choice('Apple-2', 'Apple', 12), choice('apple-1', 'Apple', 1)]
    expect(orderChoicesAlphabetically(choices).map((option) => option.value)).toEqual(['Apple-2', 'apple', 'apple-1', 'zebra'])
  })

  it('never mutates the input choices in place', () => {
    const years = [choice('2021', '2021'), choice('2025', '2025')]
    const counts = [choice('react', 'React', 5), choice('go', 'Go', 9)]
    const alpha = [...counts]
    orderYearChoicesDescending(years)
    orderChoicesByOccurrence(counts)
    orderChoicesAlphabetically(alpha)
    expect(years.map((option) => option.value)).toEqual(['2021', '2025'])
    expect(counts.map((option) => option.value)).toEqual(['react', 'go'])
    expect(alpha.map((option) => option.value)).toEqual(['react', 'go'])
  })
})
