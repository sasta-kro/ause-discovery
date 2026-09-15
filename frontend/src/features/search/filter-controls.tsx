import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import styles from '../../App.module.css'
import { orderChoicesAlphabetically, orderChoicesByOccurrence, orderYearChoicesDescending, type OptionOrderMode } from './filter-ordering'

export type FilterChoice = {
  value: string
  label: string
  count?: number
}

// FacetDisclosure ordering intent. Fixed keeps the caller's explicit order,
// year-desc sorts Academic years numerically newest first, and toggleable
// lets the visitor switch between occurrence and alphabetical presentation.
export type FacetOrder = 'fixed' | 'year-desc' | 'toggleable'

type FacetDisclosureProps = {
  defaultOpen?: boolean
  label: string
  options: FilterChoice[]
  order?: FacetOrder
  searchLabel?: string
  selected: string[]
  onToggle: (value: string) => void
}

export function FacetDisclosure({ defaultOpen = false, label, options, order = 'fixed', searchLabel, selected, onToggle }: FacetDisclosureProps) {
  const { t } = useTranslation()
  const [isOpen, setIsOpen] = useState(defaultOpen)
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<OptionOrderMode>('occurrence')
  // Set only by a direct summary interaction on a closed disclosure, so
  // initial rendering, defaultOpen, restoration, and closing never focus.
  const pendingFocus = useRef(false)
  const searchInputRef = useRef<HTMLInputElement | null>(null)

  const orderedOptions = useMemo(() => {
    if (order === 'year-desc') return orderYearChoicesDescending(options)
    if (order === 'toggleable') return mode === 'occurrence' ? orderChoicesByOccurrence(options) : orderChoicesAlphabetically(options)
    return options
  }, [order, mode, options])
  const filteredOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    if (!normalizedQuery) return orderedOptions
    return orderedOptions.filter((option) => option.label.toLocaleLowerCase().includes(normalizedQuery))
  }, [orderedOptions, query])

  useEffect(() => {
    if (!isOpen || !pendingFocus.current) return
    pendingFocus.current = false
    const element = searchInputRef.current
    if (!element) return
    element.focus()
    const end = element.value.length
    element.setSelectionRange(end, end)
  }, [isOpen])

  const markUserOpen = () => {
    if (!isOpen) pendingFocus.current = true
  }

  const nextModeLabel = mode === 'occurrence' ? t('search.orderAlphabetical') : t('search.orderMostCommon')
  const modeLabel = mode === 'occurrence' ? t('search.orderMostCommon') : t('search.orderAlphabetical')
  const orderButtonLabel = t('search.orderToggleLabel', { label, mode: modeLabel, next: nextModeLabel.toLocaleLowerCase() })

  return <details className={styles.filterDisclosure} open={isOpen} onToggle={(event) => setIsOpen(event.currentTarget.open)}>
    <summary onClick={markUserOpen} onKeyDown={(event) => handleSummaryKeyDown(event, markUserOpen, setIsOpen)}><span>{label}</span>{selected.length ? <span className={styles.filterCount}>{selected.length}</span> : null}</summary>
    <div className={styles.filterDisclosureBody}>
      {searchLabel || order === 'toggleable' ? <div className={styles.filterSearchRow}>
        {searchLabel ? <input
          aria-label={searchLabel}
          className={styles.filterSearch}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={searchLabel}
          ref={searchInputRef}
          type="search"
          value={query}
        /> : null}
        {order === 'toggleable' ? <button
          aria-label={orderButtonLabel}
          aria-pressed={mode === 'alphabetical'}
          className={styles.filterOrderToggle}
          onClick={() => setMode(mode === 'occurrence' ? 'alphabetical' : 'occurrence')}
          title={orderButtonLabel}
          type="button"
        >
          <svg aria-hidden="true" focusable="false" height="14" viewBox="0 0 16 16" width="14">
            {mode === 'occurrence'
              ? <path d="M2 13h3V6H2v7Zm4.5 0h3V3h-3v10ZM11 13h3V9h-3v4Z" fill="currentColor" />
              : <path d="M2.6 4.6 6 1.2l3.4 3.4-.9.9L6.6 3.6V10h-1.2V3.6L3.5 5.5l-.9-.9ZM6 11.5l3.4 3.4 3.4-3.4-.9-.9-1.9 1.9V7h-1.2v5.9l-1.9-1.9-.9.9Z" fill="currentColor" />}
          </svg>
        </button> : null}
      </div> : null}
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

// Enter and Space activation is handled directly with a prevented default,
// because browsers synthesize a click for these keys on summary while test
// environments and some assistive activations do not. Both paths mark the
// same one-shot focus intention, and the state toggle happens exactly once.
function handleSummaryKeyDown(event: KeyboardEvent<HTMLElement>, markUserOpen: () => void, setIsOpen: (update: (current: boolean) => boolean) => void) {
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  markUserOpen()
  setIsOpen((current) => !current)
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
