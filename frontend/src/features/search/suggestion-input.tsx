import { useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { matchSuggestions, removeSuggestionRange, type Suggestion, type SuggestionDef, type SuggestionInputState } from './suggestions'
import type { SearchState } from './state'
import styles from '../../App.module.css'

// SearchSuggestionInput is the editable combobox over the public search
// input. It owns only interaction state: popup visibility, the highlighted
// option, one-stage dismissal, and caret eligibility. Applied filters and
// the remaining free text always flow through the parent's single state
// transition. Tab accepts the highlighted suggestion while the popup is
// open; Enter always submits the draft as free text; Escape closes first
// and leaves the input on the second press.
export function SearchSuggestionInput({
  value,
  suggestionsDefs,
  applied,
  placeholder,
  onChange,
  onAccept,
  onSearch,
}: {
  value: string
  suggestionsDefs: SuggestionDef[]
  applied: SearchState
  placeholder: string
  onChange: (value: string) => void
  onAccept: (suggestion: Suggestion, remainingText: string) => void
  onSearch: () => void
}) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const listboxId = useId()
  const helpId = useId()
  const [focused, setFocused] = useState(false)
  const [collapsedCaret, setCollapsedCaret] = useState(true)
  const [caretAtEnd, setCaretAtEnd] = useState(true)
  const [activeIndex, setActiveIndex] = useState(0)
  // The exact input state suppressed by the first Escape stage or a search
  // submission; any text or caret change clears suppression.
  const [suppressed, setSuppressed] = useState<string | null>(null)
  const [escapedOnce, setEscapedOnce] = useState(false)

  const currentInputState: SuggestionInputState = useMemo(() => ({ text: value, focused, collapsedCaret, caretAtEnd }), [value, focused, collapsedCaret, caretAtEnd])
  const suggestions = useMemo(() => {
    const matches = matchSuggestions(suggestionsDefs, currentInputState, applied)
    if (suppressed !== null && suppressed === signature(currentInputState)) return []
    return matches
  }, [suggestionsDefs, currentInputState, applied, suppressed])

  const open = focused && suggestions.length > 0
  const activeOptionIndex = open ? Math.min(activeIndex, suggestions.length - 1) : 0
  const active = open ? suggestions[activeOptionIndex] : undefined

  const refreshCaret = (element: HTMLInputElement) => {
    const collapsed = element.selectionStart === element.selectionEnd
    setCollapsedCaret(collapsed)
    setCaretAtEnd(collapsed && element.selectionStart === element.value.length)
  }

  const handleChange = (element: HTMLInputElement) => {
    setSuppressed(null)
    setEscapedOnce(false)
    setActiveIndex(0)
    refreshCaret(element)
    onChange(element.value)
  }

  const accept = (suggestion: Suggestion) => {
    const remaining = removeSuggestionRange(value, suggestion.start, suggestion.end)
    setSuppressed(null)
    setEscapedOnce(false)
    setActiveIndex(0)
    setCaretAtEnd(true)
    setCollapsedCaret(true)
    onAccept(suggestion, remaining)
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    // Never accept or navigate while an input method composition is active.
    if (event.nativeEvent.isComposing) return
    if (event.key === 'Enter') {
      // Enter always searches the complete draft, never accepts a
      // suggestion, and closes the popup as part of submission.
      event.preventDefault()
      setSuppressed(signature(currentInputState))
      setEscapedOnce(false)
      onSearch()
      return
    }
    if (open) {
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        setActiveIndex(Math.min(activeOptionIndex + 1, suggestions.length - 1))
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        setActiveIndex(Math.max(activeOptionIndex - 1, 0))
        return
      }
      if (event.key === 'Tab' && active) {
        event.preventDefault()
        accept(active)
        return
      }
      if (event.key === 'Escape') {
        event.preventDefault()
        setSuppressed(signature(currentInputState))
        setEscapedOnce(true)
        return
      }
    } else if (event.key === 'Escape' && escapedOnce && suppressed === signature(currentInputState)) {
      event.preventDefault()
      setEscapedOnce(false)
      inputRef.current?.blur()
    }
  }

  return <div className={styles.suggestionWrapper}>
    <input
      ref={inputRef}
      aria-autocomplete="list"
      aria-controls={listboxId}
      aria-describedby={helpId}
      aria-expanded={open}
      aria-activedescendant={active ? optionId(listboxId, active.id) : undefined}
      role="combobox"
      autoComplete="off"
      id="search-query"
      placeholder={placeholder}
      value={value}
      onBlur={() => setFocused(false)}
      onChange={(event) => handleChange(event.target)}
      onClick={(event) => refreshCaret(event.currentTarget)}
      onFocus={(event) => { setFocused(true); refreshCaret(event.currentTarget) }}
      onKeyDown={handleKeyDown}
      onSelect={(event) => refreshCaret(event.currentTarget)}
    />
    {open ? <ul aria-label={t('search.suggestionsLabel')} className={styles.suggestionList} id={listboxId} role="listbox">
      {suggestions.map((suggestion, index) => <li
        aria-selected={index === activeOptionIndex}
        className={index === activeOptionIndex ? styles.suggestionOptionActive : styles.suggestionOption}
        id={optionId(listboxId, suggestion.id)}
        key={suggestion.id}
        onMouseDown={(event) => event.preventDefault()}
        onClick={() => accept(suggestion)}
        role="option"
      >
        <span className={styles.suggestionValue}>{suggestion.valueLabel}</span>
        <span className={styles.suggestionDimension}>{suggestion.dimensionLabel}</span>
        {index === activeOptionIndex ? <span aria-hidden="true" className={styles.suggestionHint}><kbd className={styles.keycap}>Tab</kbd> {t('search.suggestionSelectHint')}</span> : null}
      </li>)}
    </ul> : null}
    <p className={styles.suggestionHelp} id={helpId}>{t('search.suggestionHelp')}</p>
  </div>
}

// signature captures the exact draft and caret state one dismissal covers,
// so any text or caret change reopens suggestions.
function signature(input: SuggestionInputState): string {
  return `${input.text}|${input.collapsedCaret}|${input.caretAtEnd}`
}

function optionId(listboxId: string, suggestionId: string): string {
  return `${listboxId}-option-${suggestionId.replaceAll(/[^a-zA-Z0-9_-]/g, '_')}`
}
