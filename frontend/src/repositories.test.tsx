// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'
import i18n from './app/i18n'
import { ProjectRepositories } from './app-shell'

afterEach(cleanup)

const links = [
  { url: 'https://github.com/williampoch/sp1', primary: true, availability: 'accessible', checked_at: '2026-09-11T07:32:27Z' },
  { url: 'https://github.com/williampoch/sp1website', primary: false, availability: 'not_accessible', checked_at: '2026-09-11T07:32:30Z' },
]

describe('Project repositories section', () => {
  it('renders nothing without repository links', () => {
    const { container } = render(<I18nextProvider i18n={i18n}><ProjectRepositories links={[]} /></I18nextProvider>)
    expect(container.querySelector('section')).toBeNull()
  })

  it('renders every link with security attributes, availability, and the check date', () => {
    render(<I18nextProvider i18n={i18n}><ProjectRepositories links={links} /></I18nextProvider>)
    const anchors = screen.getAllByRole('link')
    expect(anchors).toHaveLength(2)
    for (const anchor of anchors) {
      expect(anchor.getAttribute('rel')).toBe('noopener noreferrer')
      expect(anchor.getAttribute('target')).toBe('_blank')
    }
    expect(anchors[0].getAttribute('href')).toBe('https://github.com/williampoch/sp1')
    expect(anchors[0].textContent).toBe('https://github.com/williampoch/sp1')
    expect(screen.getByText('Primary repository')).toBeTruthy()
    expect(screen.getByText('Accessible')).toBeTruthy()
    expect(screen.getByText('Not accessible (private or deleted)')).toBeTruthy()
    expect(screen.getAllByText(/^Last checked/)).toHaveLength(2)
  })

  it('omits the primary label for a single repository', () => {
    render(<I18nextProvider i18n={i18n}><ProjectRepositories links={[links[0]]} /></I18nextProvider>)
    expect(screen.queryByText('Primary repository')).toBeNull()
  })

  it('keeps every link clickable regardless of availability', () => {
    render(<I18nextProvider i18n={i18n}><ProjectRepositories links={links} /></I18nextProvider>)
    const unavailable = screen.getAllByRole('link')[1] as HTMLAnchorElement
    expect(unavailable.getAttribute('aria-disabled')).toBeNull()
    expect(unavailable.className).not.toContain('disabled')
  })
})
