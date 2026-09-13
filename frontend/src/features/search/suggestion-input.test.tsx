// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import type { SearchState } from './state'
import { SearchSuggestionInput } from './suggestion-input'
import { buildSuggestionDefs, type SuggestionSource } from './suggestions'

afterEach(cleanup)

const sources: SuggestionSource[] = [
  { stateField: 'academic_year', dimensionLabel: 'Academic year', cardinality: 'scalar', choices: [{ value: '2025', label: '2025' }] },
  { stateField: 'semester', dimensionLabel: 'Semester', cardinality: 'scalar', choices: [{ value: 'first', label: 'First semester' }] },
  { stateField: 'category_key', dimensionLabel: 'Category', cardinality: 'multiple', choices: [{ value: 'game', label: 'Game' }, { value: 'go', label: 'Go' }] },
  { stateField: 'platform_key', dimensionLabel: 'Platform', cardinality: 'multiple', choices: [{ value: 'game', label: 'Game' }] },
]
const defs = buildSuggestionDefs(sources)
const applied: SearchState = { limit: 20 }

function setup(value: string) {
  const props = {
    value,
    suggestionsDefs: defs,
    applied,
    placeholder: 'Search',
    onChange: vi.fn(),
    onAccept: vi.fn(),
    onSearch: vi.fn(),
  }
  const view = render(<I18nextProvider i18n={i18n}><SearchSuggestionInput {...props} /></I18nextProvider>)
  const input = screen.getByPlaceholderText('Search') as HTMLInputElement
  return { ...view, input, props }
}

// Real focus plus an explicit end caret, mirroring a user who clicked or
// typed to the end of the input. fireEvent.focus alone never moves
// document.activeElement in jsdom, and jsdom initializes selection lazily.
function focusAtEnd(input: HTMLInputElement) {
  input.focus()
  input.setSelectionRange(input.value.length, input.value.length)
  fireEvent.select(input)
}

// A controlled text change: the DOM event fires first, then the parent
// state update arrives through a rerender with the new value.
function typeText(view: { input: HTMLInputElement; rerender: typeof render['rerender']; props: ReturnType<typeof setup>['props'] }, value: string) {
  fireEvent.change(view.input, { target: { value } })
  view.rerender(<I18nextProvider i18n={i18n}><SearchSuggestionInput {...view.props} value={value} /></I18nextProvider>)
}

describe('search suggestion combobox', () => {
  it('opens suggestions on focus without searching', () => {
    const { input, props } = setup('gam')
    focusAtEnd(input)
    expect(screen.getByRole('listbox')).toBeTruthy()
    expect(props.onSearch).not.toHaveBeenCalled()
    expect(props.onChange).not.toHaveBeenCalled()
  })

  it('keeps ARIA combobox state synchronized', () => {
    const { input } = setup('gam')
    focusAtEnd(input)
    expect(input.getAttribute('role')).toBe('combobox')
    expect(input.getAttribute('aria-autocomplete')).toBe('list')
    expect(input.getAttribute('aria-expanded')).toBe('true')
    const listbox = screen.getByRole('listbox')
    expect(input.getAttribute('aria-controls')).toBe(listbox.id)
    const options = screen.getAllByRole('option')
    expect(options[0].getAttribute('aria-selected')).toBe('true')
    expect(options[1].getAttribute('aria-selected')).toBe('false')
    expect(input.getAttribute('aria-activedescendant')).toBe(options[0].id)
  })

  it('closes when no controlled value matches and reopens after correction', () => {
    const { input, rerender, props } = setup('gmae')
    focusAtEnd(input)
    expect(screen.queryByRole('listbox')).toBeNull()
    rerender(<I18nextProvider i18n={i18n}><SearchSuggestionInput {...props} value="gam" /></I18nextProvider>)
    expect(screen.getByRole('listbox')).toBeTruthy()
  })

  it('clamps arrow navigation at both boundaries', () => {
    const { input } = setup('gam')
    focusAtEnd(input)
    const options = () => screen.getAllByRole('option')
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(options()[1].getAttribute('aria-selected')).toBe('true')
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    // Two Game dimensions: index 1 is the last option.
    expect(options()[1].getAttribute('aria-selected')).toBe('true')
    fireEvent.keyDown(input, { key: 'ArrowUp' })
    expect(options()[0].getAttribute('aria-selected')).toBe('true')
    fireEvent.keyDown(input, { key: 'ArrowUp' })
    expect(options()[0].getAttribute('aria-selected')).toBe('true')
  })

  it('accepts one suggestion with Tab, keeping focus and preserving remaining text', () => {
    const { input, props } = setup('best gam')
    focusAtEnd(input)
    fireEvent.keyDown(input, { key: 'Tab' })
    expect(props.onAccept).toHaveBeenCalledTimes(1)
    const [suggestion, remaining] = props.onAccept.mock.calls[0]
    expect(suggestion.id).toBe('category_key:game')
    expect(remaining).toBe('best')
    expect(document.activeElement).toBe(input)
    expect(props.onSearch).not.toHaveBeenCalled()
  })

  it('converts only the highlighted collision option', () => {
    const { input, props } = setup('gam')
    focusAtEnd(input)
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Tab' })
    const [suggestion] = props.onAccept.mock.calls[0]
    expect(suggestion.id).toBe('platform_key:game')
  })

  it('leaves Tab to the browser when no popup is open', () => {
    const { input, props } = setup('gmae')
    focusAtEnd(input)
    fireEvent.keyDown(input, { key: 'Tab' })
    expect(props.onAccept).not.toHaveBeenCalled()
    expect(props.onSearch).not.toHaveBeenCalled()
  })

  it('submits free text with Enter even while a suggestion is highlighted', () => {
    const { input, props } = setup('gam')
    focusAtEnd(input)
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(props.onSearch).toHaveBeenCalledTimes(1)
    expect(props.onAccept).not.toHaveBeenCalled()
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('applies the two Escape stages exactly', async () => {
    const view = setup('gam')
    focusAtEnd(view.input)
    fireEvent.keyDown(view.input, { key: 'Escape' })
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(document.activeElement).toBe(view.input)

    // Any text change clears suppression and reopens suggestions.
    typeText(view, 'go')
    expect(screen.getByRole('listbox')).toBeTruthy()

    // Dismiss again, then a second Escape with unchanged input blurs.
    fireEvent.keyDown(view.input, { key: 'Escape' })
    fireEvent.keyDown(view.input, { key: 'Escape' })
    await waitFor(() => expect(document.activeElement).not.toBe(view.input))
  })

  it('does nothing on Escape with no popup and no prior dismissal', () => {
    const { input } = setup('gmae')
    focusAtEnd(input)
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(document.activeElement).toBe(input)
    expect(input.value).toBe('gmae')
  })

  it('accepts a pointer selection exactly once without a blur race', async () => {
    const user = userEvent.setup()
    const { input, props } = setup('gam')
    focusAtEnd(input)
    const option = screen.getAllByRole('option')[1]
    await user.pointer([{ keys: '[MouseLeft>]', target: option }, { keys: '[/MouseLeft]' }])
    expect(document.activeElement).toBe(input)
    expect(props.onAccept).toHaveBeenCalledTimes(1)
    expect(props.onAccept.mock.calls[0][0].id).toBe('platform_key:game')
  })

  it('ignores keyboard handling during input method composition', () => {
    const { input, props } = setup('gam')
    focusAtEnd(input)
    const composing = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
    Object.defineProperty(composing, 'isComposing', { value: true })
    fireEvent(input, composing)
    expect(props.onAccept).not.toHaveBeenCalled()
    expect(screen.getByRole('listbox')).toBeTruthy()
  })

  it('suppresses suggestions when the caret moves away from the end', () => {
    const { input } = setup('best gam')
    focusAtEnd(input)
    expect(screen.getByRole('listbox')).toBeTruthy()
    input.setSelectionRange(2, 2)
    fireEvent.select(input)
    expect(screen.queryByRole('listbox')).toBeNull()
    input.setSelectionRange(8, 8)
    fireEvent.select(input)
    expect(screen.getByRole('listbox')).toBeTruthy()
  })

  it('closes the popup on blur', () => {
    const { input } = setup('gam')
    focusAtEnd(input)
    expect(screen.getByRole('listbox')).toBeTruthy()
    fireEvent.blur(input)
    expect(screen.queryByRole('listbox')).toBeNull()
  })
})
