import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import styles from '../../App.module.css'

export type FilterChoice = {
  value: string
  label: string
  count?: number
}

type FacetDisclosureProps = {
  defaultOpen?: boolean
  label: string
  options: FilterChoice[]
  searchLabel?: string
  selected: string[]
  onToggle: (value: string) => void
}

export function FacetDisclosure({ defaultOpen = false, label, options, searchLabel, selected, onToggle }: FacetDisclosureProps) {
  const { t } = useTranslation()
  const [isOpen, setIsOpen] = useState(defaultOpen)
  const [query, setQuery] = useState('')
  const filteredOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    if (!normalizedQuery) return options
    return options.filter((option) => option.label.toLocaleLowerCase().includes(normalizedQuery))
  }, [options, query])

  return <details className={styles.filterDisclosure} open={isOpen} onToggle={(event) => setIsOpen(event.currentTarget.open)}>
    <summary><span>{label}</span>{selected.length ? <span className={styles.filterCount}>{selected.length}</span> : null}</summary>
    <div className={styles.filterDisclosureBody}>
      {searchLabel ? <input
        aria-label={searchLabel}
        className={styles.filterSearch}
        onChange={(event) => setQuery(event.target.value)}
        placeholder={searchLabel}
        type="search"
        value={query}
      /> : null}
      <div className={styles.filterChoices} role="group" aria-label={`${label} options`}>
        {filteredOptions.map((option) => {
          const active = selected.includes(option.value)
          return <button
            aria-pressed={active}
            className={`${styles.filterChoice} ${active ? styles.filterChoiceSelected : ''}`}
            key={option.value}
            onClick={() => onToggle(option.value)}
            type="button"
          >
            <span>{option.label}</span>
            {option.count !== undefined ? <span className={styles.filterChoiceCount}>{option.count}</span> : null}
            <span className={styles.filterChoiceAction}>{active ? t('search.selected') : t('search.add')}</span>
          </button>
        })}
        {filteredOptions.length === 0 ? <p className={styles.filterEmpty}>{t('search.noMatchingOptions')}</p> : null}
      </div>
    </div>
  </details>
}

type StudentIdDisclosureProps = {
  value?: string
  onApply: (value: string | undefined) => void
}

export function StudentIdDisclosure({ value, onApply }: StudentIdDisclosureProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState(value ?? '')
  const valid = /^\d{7}$/.test(draft)

  useEffect(() => setDraft(value ?? ''), [value])

  return <details className={styles.filterDisclosure}>
    <summary><span>{t('fields.studentId')}</span>{value ? <span className={styles.filterCount}>1</span> : null}</summary>
    <form className={styles.studentIdFilter} onSubmit={(event) => { event.preventDefault(); if (valid) onApply(draft) }}>
      <label className="sr-only" htmlFor="student-id-filter">{t('search.studentIdLabel')}</label>
      <input
        id="student-id-filter"
        inputMode="numeric"
        maxLength={7}
        onChange={(event) => setDraft(event.target.value.replace(/\D/g, '').slice(0, 7))}
        placeholder={t('search.studentIdLabel')}
        value={draft}
      />
      <button disabled={!valid} type="submit">{t('search.set')}</button>
    </form>
  </details>
}

type SelectedFilterChipProps = {
  label: string
  onRemove: () => void
}

export function SelectedFilterChip({ label, onRemove }: SelectedFilterChipProps) {
  const { t } = useTranslation()
  return <span className={styles.selectedFilterChip}>{label}<button aria-label={t('search.removeFilter', { label })} onClick={onRemove} type="button">×</button></span>
}

export function projectInitials(title: string): string {
  const words = title.match(/[\p{L}\p{N}]+/gu) ?? []
  if (words.length === 0) return 'AUS'
  if (words.length === 1) return words[0].slice(0, 3).toLocaleUpperCase()
  return words.slice(0, 3).map((word) => word[0]).join('').toLocaleUpperCase()
}

export function projectIdentityVariant(index: number): 'red' | 'purple' {
  return index % 3 === 1 ? 'purple' : 'red'
}
