// @vitest-environment jsdom
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { FacetDisclosure, type FilterChoice } from './filter-controls'

afterEach(cleanup)

const options: FilterChoice[] = [
  { value: 'kotlin', label: 'Kotlin', count: 0 },
  { value: 'react', label: 'React', count: 56 },
  { value: 'nodejs', label: 'Node.js', count: 41 },
  { value: 'uncounted', label: 'Uncounted' },
  { value: 'go', label: 'Go', count: 1 },
]

function choiceButtons(): HTMLButtonElement[] {
  return within(screen.getByRole('group', { name: 'Technology options' })).getAllByRole('button')
}

function choiceLabels(): string[] {
  return choiceButtons().map((button) => button.textContent ?? '')
}

function mountToggleable(overrides: Partial<Parameters<typeof FacetDisclosure>[0]> = {}) {
  const onToggle = vi.fn()
  const props = { label: 'Technology', options, order: 'toggleable' as const, searchLabel: 'Find technology', selected: [], onToggle, ...overrides }
  const view = render(<I18nextProvider i18n={i18n}><FacetDisclosure {...props} /></I18nextProvider>)
  return { ...view, props }
}

describe('facet disclosure ordering and sort toggle', () => {
  it('defaults to most common first and exposes an accessible toggle', async () => {
    const user = userEvent.setup()
    const { props } = mountToggleable()
    await user.click(screen.getByText('Technology'))
    expect(choiceLabels()).toEqual(['React56Add', 'Node.js41Add', 'Go1Add', 'Kotlin0Add', 'UncountedAdd'])
    const toggle = screen.getByRole('button', { name: 'Technology order: Most common first. Change to alphabetical.' })
    expect(toggle.getAttribute('title')).toBe('Technology order: Most common first. Change to alphabetical.')
    expect(toggle.getAttribute('aria-pressed')).toBe('false')
    expect(props.onToggle).not.toHaveBeenCalled()
  })

  it('switches to alphabetical and back while staying open', async () => {
    const user = userEvent.setup()
    mountToggleable()
    await user.click(screen.getByText('Technology'))
    const toggle = () => screen.getByRole('button', { name: /Technology order:/ })
    await user.click(toggle())
    expect(choiceLabels()).toEqual(['Go1Add', 'Kotlin0Add', 'Node.js41Add', 'React56Add', 'UncountedAdd'])
    expect(toggle().getAttribute('aria-pressed')).toBe('true')
    expect(toggle().getAttribute('aria-label')).toBe('Technology order: Alphabetical. Change to most common first.')
    expect(screen.getByText('Technology').closest('details')?.open).toBe(true)
    await user.click(toggle())
    expect(choiceLabels()).toEqual(['React56Add', 'Node.js41Add', 'Go1Add', 'Kotlin0Add', 'UncountedAdd'])
  })

  it('keeps one group mode independent of another', async () => {
    const user = userEvent.setup()
    render(<I18nextProvider i18n={i18n}>
      <FacetDisclosure label="Technology" options={options} order="toggleable" searchLabel="Find technology" selected={[]} onToggle={vi.fn()} />
      <FacetDisclosure label="Domain" options={options} order="toggleable" searchLabel="Find domain" selected={[]} onToggle={vi.fn()} />
    </I18nextProvider>)
    await user.click(screen.getByText('Technology'))
    await user.click(screen.getByRole('button', { name: 'Technology order: Most common first. Change to alphabetical.' }))
    await user.click(screen.getByText('Domain'))
    const domainToggle = screen.getByRole('button', { name: 'Domain order: Most common first. Change to alphabetical.' })
    expect(domainToggle.getAttribute('aria-pressed')).toBe('false')
    const group = (name: string) => within(screen.getByRole('group', { name })).getAllByRole('button')
    expect(group('Technology options')[0].textContent).toContain('Go')
    expect(group('Domain options')[0].textContent).toContain('React')
  })

  it('preserves the mode across closing and reopening while mounted', async () => {
    const user = userEvent.setup()
    mountToggleable()
    await user.click(screen.getByText('Technology'))
    await user.click(screen.getByRole('button', { name: /Technology order:/ }))
    await user.click(screen.getByText('Technology'))
    expect(screen.getByText('Technology').closest('details')?.open).toBe(false)
    await user.click(screen.getByText('Technology'))
    expect(choiceLabels()[0]).toBe('Go1Add')
  })

  it('keeps the active order mode while filtering locally', async () => {
    const user = userEvent.setup()
    mountToggleable()
    await user.click(screen.getByText('Technology'))
    await user.click(screen.getByRole('button', { name: /Technology order:/ }))
    await user.type(screen.getByLabelText('Find technology'), 'o')
    expect(choiceLabels().every((label) => label.toLocaleLowerCase().includes('o'))).toBe(true)
    expect(choiceLabels()[0]).toBe('Go1Add')
  })

  it('re-sorts occurrence mode from updated counts without clearing the local query', async () => {
    const user = userEvent.setup()
    const view = mountToggleable()
    await user.click(screen.getByText('Technology'))
    await user.type(screen.getByLabelText('Find technology'), 'o')
    const reordered: FilterChoice[] = [
      { value: 'go', label: 'Go', count: 90 },
      { value: 'react', label: 'React', count: 3 },
      { value: 'nodejs', label: 'Node.js', count: 41 },
      { value: 'kotlin', label: 'Kotlin', count: 0 },
      { value: 'uncounted', label: 'Uncounted' },
    ]
    view.rerender(<I18nextProvider i18n={i18n}><FacetDisclosure {...view.props} options={reordered} /></I18nextProvider>)
    expect((screen.getByLabelText('Find technology') as HTMLInputElement).value).toBe('o')
    expect(choiceLabels()[0]).toBe('Go90Add')
  })

  it('keeps selection bound to the correct value after reordering', async () => {
    const user = userEvent.setup()
    const onToggle = vi.fn()
    mountToggleable({ selected: ['react'], onToggle })
    await user.click(screen.getByText('Technology'))
    expect(choiceButtons()[0].getAttribute('aria-pressed')).toBe('true')
    await user.click(screen.getByRole('button', { name: /Technology order:/ }))
    const buttons = choiceButtons()
    const react = buttons.find((button) => button.textContent?.includes('React'))!
    expect(react.getAttribute('aria-pressed')).toBe('true')
    expect(buttons[0].getAttribute('aria-pressed')).toBe('false')
    await user.click(react)
    expect(onToggle).toHaveBeenCalledWith('react')
  })

  it('sorts academic years newest first without a toggle', async () => {
    const user = userEvent.setup()
    render(<I18nextProvider i18n={i18n}><FacetDisclosure label="Academic year" options={[{ value: '2021', label: '2021' }, { value: '2025', label: '2025' }, { value: '2016', label: '2016' }]} order="year-desc" searchLabel="Find a year" selected={[]} onToggle={vi.fn()} /></I18nextProvider>)
    await user.click(screen.getByText('Academic year'))
    const labels = within(screen.getByRole('group', { name: 'Academic year options' })).getAllByRole('button').map((button) => button.textContent)
    expect(labels).toEqual(['2025Add', '2021Add', '2016Add'])
    expect(screen.queryByRole('button', { name: /order:/ })).toBeNull()
  })

  it('keeps the caller order for fixed dimensions without a search input', async () => {
    const user = userEvent.setup()
    render(<I18nextProvider i18n={i18n}><FacetDisclosure label="Semester" options={[{ value: 'first', label: 'First semester' }, { value: 'second', label: 'Second semester' }, { value: 'summer', label: 'Summer semester' }]} order="fixed" selected={[]} onToggle={vi.fn()} /></I18nextProvider>)
    await user.click(screen.getByText('Semester'))
    const labels = within(screen.getByRole('group', { name: 'Semester options' })).getAllByRole('button').map((button) => button.textContent)
    expect(labels).toEqual(['First semesterAdd', 'Second semesterAdd', 'Summer semesterAdd'])
    expect(screen.queryByRole('searchbox')).toBeNull()
  })
})

describe('facet disclosure expansion focus', () => {
  it('focuses the local input after a pointer-opened searchable disclosure', async () => {
    const user = userEvent.setup()
    mountToggleable()
    await user.click(screen.getByText('Technology'))
    const input = screen.getByLabelText('Find technology') as HTMLInputElement
    expect(document.activeElement).toBe(input)
  })

  it('focuses the local input after a keyboard-opened searchable disclosure', async () => {
    const user = userEvent.setup()
    mountToggleable()
    screen.getByText('Technology').closest('summary')!.focus()
    await user.keyboard('{Enter}')
    const input = screen.getByLabelText('Find technology') as HTMLInputElement
    expect(document.activeElement).toBe(input)
  })

  it('places the caret at the end of an existing local query without clearing it', async () => {
    const user = userEvent.setup()
    mountToggleable()
    await user.click(screen.getByText('Technology'))
    const input = screen.getByLabelText('Find technology') as HTMLInputElement
    await user.type(input, 'rea')
    await user.click(screen.getByText('Technology'))
    await user.click(screen.getByText('Technology'))
    expect(document.activeElement).toBe(input)
    expect(input.value).toBe('rea')
    expect(input.selectionStart).toBe(3)
    expect(input.selectionEnd).toBe(3)
  })

  it('does not focus the input when the disclosure closes', async () => {
    const user = userEvent.setup()
    mountToggleable()
    await user.click(screen.getByText('Technology'))
    const input = screen.getByLabelText('Find technology')
    await user.click(screen.getByText('Technology'))
    expect(document.activeElement).not.toBe(input)
  })

  it('does not steal focus on initial defaultOpen rendering', () => {
    mountToggleable({ defaultOpen: true })
    expect(screen.getByLabelText('Find technology')).toBeTruthy()
    expect(document.activeElement).toBe(document.body)
  })

  it('does not create an artificial focus target for groups without a search input', async () => {
    const user = userEvent.setup()
    render(<I18nextProvider i18n={i18n}><FacetDisclosure label="Semester" options={[{ value: 'first', label: 'First semester' }]} order="fixed" selected={[]} onToggle={vi.fn()} /></I18nextProvider>)
    const summary = screen.getByText('Semester').closest('summary')!
    await user.click(summary)
    // The clicked summary keeps natural pointer focus; nothing else receives it.
    expect(document.activeElement).toBe(summary)
  })

  it('does not close the disclosure or move focus to the search input from the sort button', async () => {
    const user = userEvent.setup()
    mountToggleable()
    await user.click(screen.getByText('Technology'))
    const input = screen.getByLabelText('Find technology')
    await user.click(screen.getByRole('button', { name: /Technology order:/ }))
    expect(screen.getByText('Technology').closest('details')?.open).toBe(true)
    expect(document.activeElement).not.toBe(input)
  })
})
