import type { ReactNode } from 'react'

function escapePattern(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function highlightText(value: string, terms: string[]): ReactNode[] {
  const normalizedTerms = [...new Set(terms.map((term) => term.trim()).filter(Boolean))]
  if (normalizedTerms.length === 0) return [value]
  const expression = new RegExp(`(${normalizedTerms.map(escapePattern).join('|')})`, 'gi')
  return value.split(expression).filter(Boolean).map((part, index) => normalizedTerms.some((term) => part.toLocaleLowerCase() === term.toLocaleLowerCase())
    ? <mark key={`${part}-${index}`}>{part}</mark>
    : part)
}
