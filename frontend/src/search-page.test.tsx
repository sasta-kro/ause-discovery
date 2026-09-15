// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter, useNavigate } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from './app/i18n'
import { AppRoutes, SessionProvider } from './app-shell'

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn(),
  searchProjects: vi.fn(),
  getCatalogs: vi.fn(),
}))

vi.mock('./api/generated/sdk.gen', () => apiMocks)

const emptyFacets = { programs: [], majors: [], courses: [], academic_years: [], semesters: [], people: [], advisors: [], categories: [], platforms: [], domains: [], topics: [], technologies: [] }

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
    await user.click(screen.getByText('Semester'))
    await user.click(screen.getByRole('button', { name: /First semester.*Add/ }))
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

  it('searches, selects, and removes an independent metadata filter', async () => {
    const user = userEvent.setup()
    apiMocks.getCatalogs.mockResolvedValue({ data: {
      programs: [], majors: [], courses: [],
      taxonomy: [{ id: 'technology-react', key: 'react', dimension: 'technology', labels: { en: 'React' } }],
    } })
    apiMocks.searchProjects.mockImplementation(async () => ({ data: {
      items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')],
      page: { limit: 20 },
      facets: { ...emptyFacets, technologies: [{ key: 'react', label: 'React', count: 1 }] },
      total: 1,
    } }))
    renderSearch()
    await screen.findByRole('heading', { name: 'First Page Result' })

    await user.type(screen.getByLabelText('Find technology'), 'Rea')
    await user.click(screen.getByRole('button', { name: /React.*1.*Add/i }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ technology_key: ['react'] }) })))

    await user.click(screen.getByRole('button', { name: 'Remove Technology: React' }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ technology_key: undefined }) })))
  })

  it('adds and removes People and Advisor filters and resets paging history', async () => {
    const user = userEvent.setup()
    const personID = '018f0000-0000-7000-8000-000000000701'
    apiMocks.getCatalogs.mockResolvedValue({ data: { programs: [], majors: [], courses: [], taxonomy: [] } })
    apiMocks.searchProjects.mockImplementation(async ({ query }: { query?: { cursor?: string } }) => {
      const facets = {
        ...emptyFacets,
        people: [{ key: personID, label: 'Alex Advisor', count: 4 }],
        advisors: [{ key: personID, label: 'Alex Advisor', count: 2 }],
      }
      if (query?.cursor === 'search-cursor-2') return { data: { items: [resultItem('018f0000-0000-7000-8000-0000000000r2', 'Second Page Result')], page: { limit: 20 }, facets, total: 2 } }
      return { data: { items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')], page: { limit: 20, next_cursor: 'search-cursor-2' }, facets, total: 2 } }
    })
    renderSearch('neural')
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByRole('heading', { name: 'Second Page Result' })
    expect((screen.getByRole('button', { name: 'Previous page' }) as HTMLButtonElement).disabled).toBe(false)

    await user.click(screen.getByText('People'))
    await user.click(screen.getByRole('button', { name: /Alex Advisor.*4.*Add/ }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ cursor: undefined, person_id: [personID] }) })))
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Previous page' }) as HTMLButtonElement).disabled).toBe(true)

    await user.click(screen.getByText('Advisor'))
    await user.click(screen.getByRole('button', { name: /Alex Advisor.*2.*Add/ }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ advisor_id: [personID], person_id: [personID] }) })))

    await user.click(screen.getByRole('button', { name: 'Remove Advisor: Alex Advisor' }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ advisor_id: undefined, person_id: [personID] }) })))
    await user.click(screen.getByRole('button', { name: 'Remove People: Alex Advisor' }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ person_id: undefined }) })))
  })

  it('suggests person names without searching and applies one dimension per Tab', async () => {
    const user = userEvent.setup()
    const personID = '018f0000-0000-7000-8000-000000000701'
    apiMocks.getCatalogs.mockResolvedValue({ data: { programs: [], majors: [], courses: [], taxonomy: [] } })
    apiMocks.searchProjects.mockResolvedValue({ data: {
      items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')],
      page: { limit: 20 },
      facets: { ...emptyFacets, people: [{ key: personID, label: 'Alex Advisor', count: 4 }], advisors: [{ key: personID, label: 'Alex Advisor', count: 2 }] },
      total: 1,
    } })
    renderSearch('neural')
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()

    const input = screen.getByLabelText('Search terms') as HTMLInputElement
    const callsBeforeTyping = apiMocks.searchProjects.mock.calls.length
    await user.type(input, ' Alex')
    expect(screen.getByRole('listbox')).toBeTruthy()
    await user.type(input, ' Adv')
    expect(screen.getByRole('listbox')).toBeTruthy()
    expect(apiMocks.searchProjects.mock.calls.length).toBe(callsBeforeTyping)

    await user.type(input, 'isor')
    // Scoped to the suggestion popup: the sort select and filter panels also
    // expose option roles.
    const options = within(screen.getByRole('listbox', { name: 'Filter suggestions' })).getAllByRole('option')
    expect(options).toHaveLength(2)
    expect(options[0].textContent).toContain('People')
    expect(options[1].textContent).toContain('Advisor')

    await user.keyboard('{ArrowDown}')
    await user.keyboard('{Tab}')
    expect(apiMocks.searchProjects.mock.calls.length).toBe(callsBeforeTyping + 1)
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ advisor_id: [personID], cursor: undefined, q: 'neural' }) })))
    expect(document.activeElement).toBe(input)
    expect(screen.getByRole('button', { name: 'Remove Advisor: Alex Advisor' })).toBeTruthy()
    expect(input.value).toBe('neural')

    await user.click(screen.getByRole('button', { name: 'Remove Advisor: Alex Advisor' }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ advisor_id: undefined }) })))
    await user.type(input, ' Alex')
    const restored = within(screen.getByRole('listbox', { name: 'Filter suggestions' })).getAllByRole('option')
    expect(restored.length).toBeGreaterThan(0)
    expect(restored.some((option) => option.textContent?.includes('Advisor'))).toBe(true)
    expect(apiMocks.searchProjects.mock.calls.length).toBe(callsBeforeTyping + 2)
  })

  it('preserves preceding free text when a person suggestion is accepted', async () => {
    const user = userEvent.setup()
    const personID = '018f0000-0000-7000-8000-000000000701'
    apiMocks.getCatalogs.mockResolvedValue({ data: { programs: [], majors: [], courses: [], taxonomy: [] } })
    apiMocks.searchProjects.mockResolvedValue({ data: {
      items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')],
      page: { limit: 20 },
      facets: { ...emptyFacets, people: [{ key: personID, label: 'Alex Advisor', count: 4 }] },
      total: 1,
    } })
    renderSearch('archive')
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()

    const input = screen.getByLabelText('Search terms') as HTMLInputElement
    await user.click(input)
    await user.type(input, ' Alex Adv')
    expect(within(screen.getByRole('listbox', { name: 'Filter suggestions' })).getAllByRole('option')).toHaveLength(1)
    await user.keyboard('{Tab}')
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ person_id: [personID], q: 'archive' }) })))
    expect(input.value).toBe('archive')
    expect(screen.getByRole('button', { name: 'Remove People: Alex Advisor' })).toBeTruthy()
  })

  it('orders fixed dimensions and defaults toggleable groups to count descending without requests', async () => {
    const user = userEvent.setup()
    const personID = '018f0000-0000-7000-8000-000000000701'
    apiMocks.getCatalogs.mockResolvedValue({ data: {
      programs: [{ id: 'p-it', key: 'information_technology', label: 'Information Technology' }, { id: 'p-cs', key: 'computer_science', label: 'Computer Science' }],
      majors: [], courses: [],
      taxonomy: [
        { id: 't-react', key: 'react', dimension: 'technology', labels: { en: 'React' } },
        { id: 't-node', key: 'nodejs', dimension: 'technology', labels: { en: 'Node.js' } },
        { id: 't-go', key: 'go', dimension: 'technology', labels: { en: 'Go' } },
      ],
    } })
    apiMocks.searchProjects.mockResolvedValue({ data: {
      items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')],
      page: { limit: 20 },
      facets: {
        ...emptyFacets,
        academic_years: [{ key: '2021', count: 7 }, { key: '2025', count: 50 }, { key: '2023', count: 12 }],
        people: [{ key: personID, label: 'Alex Advisor', count: 4 }, { key: '018f0000-0000-7000-8000-000000000702', label: 'Bee Student', count: 19 }],
        programs: [{ key: 'computer_science', count: 2 }, { key: 'information_technology', count: 33 }],
        technologies: [{ key: 'go', count: 1 }, { key: 'react', count: 56 }, { key: 'nodejs', count: 41 }],
      },
      total: 1,
    } })
    renderSearch()
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()
    const callsBefore = apiMocks.searchProjects.mock.calls.length

    const groupButtons = (name: string) => within(screen.getByRole('group', { name })).getAllByRole('button')
    await user.click(screen.getByText('Academic year'))
    expect(groupButtons('Academic year options').map((button) => button.textContent)).toEqual(['202550Add', '202312Add', '20217Add'])
    expect(document.activeElement).toBe(screen.getByLabelText('Find a year'))
    expect(screen.queryByRole('button', { name: /Academic year order:/ })).toBeNull()

    await user.click(screen.getByText('Semester'))
    expect(groupButtons('Semester options').map((button) => button.textContent![0])).toEqual(['F', 'S', 'S'])
    expect(screen.queryByRole('searchbox', { name: /semester/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /Semester order:/ })).toBeNull()

    await user.click(screen.getByText('Program'))
    expect(groupButtons('Program options').map((button) => button.textContent)).toEqual(['Information Technology33Add', 'Computer Science2Add'])
    await user.click(screen.getByText('People'))
    expect(groupButtons('People options').map((button) => button.textContent)).toEqual(['Bee Student19Add', 'Alex Advisor4Add'])
    await user.click(screen.getByText('Technology'))
    expect(groupButtons('Technology options').map((button) => button.textContent)).toEqual(['React56Add', 'Node.js41Add', 'Go1Add'])
    expect(apiMocks.searchProjects.mock.calls.length).toBe(callsBefore)
  })

  it('toggles representative groups to alphabetical independently and keeps selection bound', async () => {
    const user = userEvent.setup()
    const personID = '018f0000-0000-7000-8000-000000000701'
    apiMocks.getCatalogs.mockResolvedValue({ data: {
      programs: [{ id: 'p-it', key: 'information_technology', label: 'Information Technology' }, { id: 'p-cs', key: 'computer_science', label: 'Computer Science' }],
      majors: [], courses: [],
      taxonomy: [{ id: 't-react', key: 'react', dimension: 'technology', labels: { en: 'React' } }, { id: 't-go', key: 'go', dimension: 'technology', labels: { en: 'Go' } }],
    } })
    apiMocks.searchProjects.mockResolvedValue({ data: {
      items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')],
      page: { limit: 20 },
      facets: {
        ...emptyFacets,
        people: [{ key: personID, label: 'Zeta Advisor', count: 4 }, { key: '018f0000-0000-7000-8000-000000000702', label: 'Alpha Student', count: 1 }],
        programs: [{ key: 'computer_science', count: 9 }, { key: 'information_technology', count: 2 }],
        technologies: [{ key: 'go', count: 1 }, { key: 'react', count: 56 }],
      },
      total: 1,
    } })
    renderSearch()
    expect(await screen.findByRole('heading', { name: 'First Page Result' })).toBeTruthy()
    const callsBefore = apiMocks.searchProjects.mock.calls.length
    const groupButtons = (name: string) => within(screen.getByRole('group', { name })).getAllByRole('button')

    await user.click(screen.getByText('Program'))
    await user.click(screen.getByRole('button', { name: 'Program order: Most common first. Change to alphabetical.' }))
    expect(groupButtons('Program options').map((button) => button.textContent)).toEqual(['Computer Science9Add', 'Information Technology2Add'])

    await user.click(screen.getByText('People'))
    await user.click(screen.getByRole('button', { name: 'People order: Most common first. Change to alphabetical.' }))
    expect(groupButtons('People options').map((button) => button.textContent)).toEqual(['Alpha Student1Add', 'Zeta Advisor4Add'])

    await user.click(screen.getByText('Technology'))
    expect(groupButtons('Technology options').map((button) => button.textContent)).toEqual(['React56Add', 'Go1Add'])
    expect(screen.getByRole('button', { name: 'Program order: Alphabetical. Change to most common first.' }).getAttribute('aria-pressed')).toBe('true')

    await user.click(groupButtons('People options')[0])
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ person_id: ['018f0000-0000-7000-8000-000000000702'] }) })))
    expect(apiMocks.searchProjects.mock.calls.length).toBe(callsBefore + 1)
    await user.type(screen.getByLabelText('Find a person'), 'zet')
    expect(groupButtons('People options').map((button) => button.textContent)).toEqual(['Zeta Advisor4Add'])
    expect(apiMocks.searchProjects.mock.calls.length).toBe(callsBefore + 1)
  })

  it('shows named participant facets and Semester counts while hiding paused dimensions', async () => {
    const user = userEvent.setup()
    const personID = '018f0000-0000-7000-8000-000000000701'
    apiMocks.getCatalogs.mockResolvedValue({ data: {
      programs: [],
      majors: [{ id: 'major-1', key: 'software_engineering', label: 'Software Engineering' }],
      courses: [],
      taxonomy: [{ id: 'topic-1', key: 'computer_vision', dimension: 'topic', labels: { en: 'Computer Vision' } }],
    } })
    apiMocks.searchProjects.mockResolvedValue({ data: {
      items: [resultItem('018f0000-0000-7000-8000-0000000000r1', 'First Page Result')],
      page: { limit: 20 },
      facets: {
        ...emptyFacets,
        semesters: [{ key: 'first', count: 12 }],
        people: [{ key: personID, label: 'Alex Advisor', count: 4 }],
        advisors: [{ key: personID, label: 'Alex Advisor', count: 2 }],
      },
      total: 1,
    } })
    renderSearch()
    await screen.findByRole('heading', { name: 'First Page Result' })

    expect(screen.queryByText('Major')).toBeNull()
    expect(screen.queryByText('Topic')).toBeNull()
    await user.click(screen.getByText('Semester'))
    expect(screen.getByRole('button', { name: /First semester.*12.*Add/ })).toBeTruthy()
    await user.click(screen.getByText('People'))
    expect(screen.getByRole('button', { name: /Alex Advisor.*4.*Add/ })).toBeTruthy()
    await user.click(screen.getByText('Advisor'))
    expect(screen.getByRole('button', { name: /Alex Advisor.*2.*Add/ })).toBeTruthy()
  })

  it('synchronizes the draft field when history navigation changes the URL query', async () => {
    let navigate: ((delta: number) => void) | undefined
    function NavigationProbe() {
      navigate = useNavigate()
      return null
    }
    const user = userEvent.setup()
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <MemoryRouter initialEntries={['/search']}>
            <SessionProvider><NavigationProbe /><AppRoutes /></SessionProvider>
          </MemoryRouter>
        </I18nextProvider>
      </QueryClientProvider>,
    )
    await screen.findByRole('heading', { name: 'First Page Result' })

    await user.type(screen.getByLabelText('Search terms'), 'neural')
    await user.click(screen.getByRole('button', { name: 'Search projects' }))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ q: 'neural' }) })))

    await user.clear(screen.getByLabelText('Search terms'))
    await user.type(screen.getByLabelText('Search terms'), 'abandoned draft')
    expect((screen.getByLabelText('Search terms') as HTMLInputElement).value).toBe('abandoned draft')

    navigate?.(-1)
    await waitFor(() => expect((screen.getByLabelText('Search terms') as HTMLInputElement).value).toBe(''))
    await waitFor(() => expect(apiMocks.searchProjects).toHaveBeenLastCalledWith(expect.objectContaining({ query: expect.objectContaining({ q: undefined }) })))
  })
})

describe('public search contextual sort display', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getSession.mockRejectedValue({ code: 'unauthorized', status: 401, title: 'Unauthorized', type: 'about:blank', request_id: 'test' })
    apiMocks.getCatalogs.mockResolvedValue({ data: { programs: [], majors: [], courses: [], taxonomy: [] } })
    apiMocks.searchProjects.mockImplementation(async () => {
      return { data: { items: [resultItem('018f0000-0000-7000-8000-0000000000s1', 'Sort Display Result')], page: { limit: 20 }, facets: emptyFacets, total: 1 } }
    })
  })

  it('displays Newest first for an implicit empty search without a sort parameter', async () => {
    renderSearch()
    await screen.findByText('Sort Display Result')
    expect((screen.getByLabelText('Sort results') as HTMLSelectElement).value).toBe('newest')
    expect(apiMocks.searchProjects.mock.calls[0][0].query.sort).toBeUndefined()
  })

  it('displays Most relevant for an implicit text search', async () => {
    renderSearch('vision')
    await screen.findByText('Sort Display Result')
    expect((screen.getByLabelText('Sort results') as HTMLSelectElement).value).toBe('relevance')
    expect(apiMocks.searchProjects.mock.calls[0][0].query.sort).toBeUndefined()
  })

  it('switches the implicit displayed default when text is submitted and cleared', async () => {
    renderSearch()
    await screen.findByText('Sort Display Result')
    const input = screen.getByPlaceholderText('Search title, student, advisor, or code')
    const form = input.closest('form') as HTMLFormElement
    const select = screen.getByLabelText('Sort results') as HTMLSelectElement
    fireEvent.change(input, { target: { value: 'vision' } })
    fireEvent.submit(form)
    await waitFor(() => expect((screen.getByLabelText('Sort results') as HTMLSelectElement).value).toBe('relevance'))
    fireEvent.change(input, { target: { value: '' } })
    fireEvent.submit(form)
    await waitFor(() => expect((screen.getByLabelText('Sort results') as HTMLSelectElement).value).toBe('newest'))
    expect(select.value).toBe('newest')
  })

  it('sends an explicit sort selection to the API', async () => {
    renderSearch()
    await screen.findByText('Sort Display Result')
    fireEvent.change(screen.getByLabelText('Sort results'), { target: { value: 'oldest' } })
    await waitFor(() => {
      const calls = apiMocks.searchProjects.mock.calls.map((call) => call[0].query.sort)
      expect(calls[calls.length - 1]).toBe('oldest')
    })
    expect((screen.getByLabelText('Sort results') as HTMLSelectElement).value).toBe('oldest')
  })
})
