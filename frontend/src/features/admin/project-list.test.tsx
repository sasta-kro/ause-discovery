// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { AdminProjectList } from './project-list'

const apiMocks = vi.hoisted(() => ({
  listAdminProjects: vi.fn(),
}))

vi.mock('../../api/generated/sdk.gen', () => apiMocks)

const draftProject = {
  id: '018f0000-0000-7000-8000-0000000000a1',
  title: 'Alpha Draft Project',
  status: 'draft',
  academic_year: 2026,
  abstract: null,
  semester: null,
  program: null,
  major: null,
  course: null,
  taxonomy: [],
  participations: [],
  artifacts: [],
  extension_metadata: {},
  reference_code: null,
  title_aliases: [],
  revision: 1,
  created_at: '2026-09-04T00:00:00Z',
  updated_at: '2026-09-04T00:00:00Z',
  published_at: null,
  deleted_at: null,
}

const publishedProject = { ...draftProject, id: '018f0000-0000-7000-8000-0000000000a2', title: 'Published History', status: 'published' }

function renderList() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter><AdminProjectList /></MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  )
}

describe('administrator project list', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.listAdminProjects.mockImplementation(async ({ query }: { query?: { cursor?: string; q?: string; status?: string } }) => {
      if (query?.cursor === 'cursor-2') return { data: { items: [publishedProject], page: { limit: 20 } } }
      return { data: { items: [draftProject], page: { limit: 20, next_cursor: 'cursor-2' } } }
    })
  })

  it('filters with exact parameters and pages through cursor history without duplicates', async () => {
    const user = userEvent.setup()
    renderList()
    expect(await screen.findByText('Alpha Draft Project')).toBeTruthy()

    await user.type(screen.getByLabelText('Filter by title or reference'), 'Alpha')
    await user.selectOptions(screen.getByLabelText('Status'), 'draft')
    await user.click(screen.getByRole('button', { name: 'Apply filters' }))
    await waitFor(() => expect(apiMocks.listAdminProjects).toHaveBeenCalledWith(expect.objectContaining({ query: { q: 'Alpha', status: 'draft', cursor: undefined, limit: 20 } })))

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(apiMocks.listAdminProjects).toHaveBeenCalledWith(expect.objectContaining({ query: { q: 'Alpha', status: 'draft', cursor: 'cursor-2', limit: 20 } })))
    expect(await screen.findByText('Published History')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Previous page' }))
    await waitFor(() => expect(apiMocks.listAdminProjects).toHaveBeenCalledWith(expect.objectContaining({ query: { q: 'Alpha', status: 'draft', cursor: undefined, limit: 20 } })))
    expect(await screen.findByText('Alpha Draft Project')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(apiMocks.listAdminProjects).toHaveBeenCalledWith(expect.objectContaining({ query: expect.objectContaining({ cursor: 'cursor-2' }) })))
    await user.click(screen.getByRole('button', { name: 'Clear filters' }))
    await waitFor(() => expect(apiMocks.listAdminProjects).toHaveBeenCalledWith(expect.objectContaining({ query: { q: undefined, status: undefined, cursor: undefined, limit: 20 } })))
    expect(await screen.findByText('Alpha Draft Project')).toBeTruthy()
  })

  it('shows a request failure instead of a false empty state', async () => {
    apiMocks.listAdminProjects.mockRejectedValue(new Error('unavailable'))
    renderList()
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('The request failed.')
  })

  it('shows the empty state only after a successful empty page', async () => {
    apiMocks.listAdminProjects.mockResolvedValue({ data: { items: [], page: { limit: 20 } } })
    renderList()
    expect(await screen.findByText('No Projects matched the current filters.')).toBeTruthy()
  })
})
