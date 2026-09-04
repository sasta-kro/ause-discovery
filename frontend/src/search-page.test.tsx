// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from './app/i18n'
import { AppRoutes, SessionProvider } from './app-shell'

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn(),
  searchProjects: vi.fn(),
  getCatalogs: vi.fn(),
}))

vi.mock('./api/generated/sdk.gen', () => apiMocks)

const emptyFacets = { programs: [], majors: [], courses: [], academic_years: [], people: [], categories: [], platforms: [], domains: [], topics: [], technologies: [] }

function resultItem(id: string, title: string) {
  return {
    id,
    reference_code: null,
    title,
    academic_year: 2026,
    semester: 'first',
    program: { id, key: 'computing', label: 'Computing' },
    course: { id, key: 'capstone', label: 'Capstone' },
    major: null,
    categories: [],
    platforms: [],
    people: [],
    artifact_count: 0,
    published_at: '2026-09-01T00:00:00Z',
    highlights: [{ field: 'abstract', value: 'A matching abstract excerpt about neural retrieval.' }],
  }
}

function renderSearch(initialQuery = '') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={[initialQuery ? `/search?q=${encodeURIComponent(initialQuery)}` : '/search']}>
          <SessionProvider><AppRoutes /></SessionProvider>
        </MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  )
}

describe('public search paging', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getSession.mockRejectedValue({ code: 'unauthorized', status: 401, title: 'Unauthorized', type: 'about:blank', request_id: 'test' })
    apiMocks.getCatalogs.mockResolvedValue({ data: { programs: [], majors: [], courses: [], taxonomy: [] } })
    apiMocks.searchProjects.mockImplementation(async ({ query }: { query?: { cursor?: string; q?: string; semester?: string } }) => {
      if (query?.cursor === 'search-cursor-2') return { data: { items: [resultItem('018f0000-0000-7000-8000-0000000000r2', 'Second Page Result')], page: { limit: 20 }, facets: emptyFacets, total: 2 } }
      return { data: { items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')], page: { limit: 20, next_cursor: 'search-cursor-2' }, facets: emptyFacets, total: 2 } }
    })
  })

  it('uses accurate page labels, local cursor history, and resets history after a filter change', async () => {
    const user = userEvent.setup()
    renderSearch('neural')
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()

    const previous = screen.getByRole('button', { name: 'Previous page' }) as HTMLButtonElement
    expect(previous.disabled).toBe(true)

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenCalledWith(expect.objectContaining({ query: expect.objectContaining({ cursor: 'search-cursor-2', q: 'neural' }) })))
    expect(await screen.findByRole('heading', { name: 'Second Page Result' })).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Previous page' }) as HTMLButtonElement).disabled).toBe(false)

    await user.click(screen.getByRole('button', { name: 'Previous page' }))
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('heading', { name: 'Second Page Result' })
    await user.selectOptions(screen.getByLabelText('Semester'), 'first')
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ cursor: undefined, semester: 'first' }) })))
    expect((screen.getByRole('button', { name: 'Previous page' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('does not search on every keystroke and applies the draft on submit', async () => {
    const user = userEvent.setup()
    renderSearch()
    await screen.findByRole('heading', { name: 'First Page Result' })
    const callsBefore = apiMocks.searchProjects.mock.calls.length

    await user.type(screen.getByLabelText('Search terms'), 'neural')
    expect(apiMocks.searchProjects.mock.calls.length).toBe(callsBefore)

    await user.click(screen.getByRole('button', { name: 'Search projects' }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ q: 'neural' }) })))
  })
})
