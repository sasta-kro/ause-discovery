// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { AppRoutes, SessionProvider } from '../../app-shell'

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn(),
  getCsrfToken: vi.fn(),
  getAdminProject: vi.fn(),
  getCatalogs: vi.fn(),
  listAdminPeople: vi.fn(),
  replaceProject: vi.fn(),
  deleteProject: vi.fn(),
}))

vi.mock('../../api/generated/sdk.gen', () => apiMocks)

const projectID = '018f0000-0000-7000-8000-0000000000e1'
const activeSession = { user: { id: '018f0000-0000-7000-8000-000000000001', username: 'admin', permissions: ['project.edit'] }, expires_at: '2026-09-05T00:00:00Z' }
const catalogs = { programs: [], majors: [], courses: [], taxonomy: [] }

function storedProject(revision: number) {
  return {
    id: projectID,
    reference_code: 'REF-FORM-1',
    title: 'Stored Form Project',
    abstract: null,
    academic_year: null,
    semester: null,
    program: null,
    major: null,
    course: null,
    status: 'draft',
    taxonomy: [],
    participations: [],
    artifacts: [],
    extension_metadata: {},
    title_aliases: [],
    revision,
    created_at: '2026-09-04T00:00:00Z',
    updated_at: '2026-09-04T00:00:00Z',
    published_at: null,
    deleted_at: null,
  }
}

function renderAt(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={[path]}>
          <SessionProvider><AppRoutes /></SessionProvider>
        </MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  )
}

describe('administrator project form boundaries', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getSession.mockResolvedValue({ data: activeSession })
    apiMocks.getCsrfToken.mockResolvedValue({ data: { token: 'csrf-token' } })
    apiMocks.getAdminProject.mockResolvedValue({ data: storedProject(2) })
    apiMocks.getCatalogs.mockResolvedValue({ data: catalogs })
    apiMocks.listAdminPeople.mockResolvedValue({ data: { items: [], page: { limit: 100 } } })
  })

  it('blocks the form behind dependency loading and failure with an explicit retry', async () => {
    apiMocks.getCatalogs.mockReturnValue(new Promise(() => {}))
    renderAt(`/admin/projects/${projectID}/edit`)
    await waitFor(() => expect(screen.getByRole('status').textContent).toContain('Loading the record and reference data'))
    expect(screen.queryByRole('button', { name: 'Save draft' })).toBeNull()

    apiMocks.getCatalogs.mockRejectedValue(new Error('catalogs down'))
    renderAt(`/admin/projects/${projectID}/edit`)
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('Academic and classification catalogs could not be loaded')
    expect(screen.queryByRole('button', { name: 'Save draft' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy()

    apiMocks.getCatalogs.mockResolvedValue({ data: catalogs })
    await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByRole('button', { name: 'Save draft' })).toBeTruthy()
    expect((screen.getByLabelText('Project title') as HTMLInputElement).value).toBe('Stored Form Project')
  })

  it('reuses the revision returned by a successful edit for the next save', async () => {
    const user = userEvent.setup()
    apiMocks.replaceProject.mockImplementation(async ({ body }: { body?: { title?: string | null } }) => ({ data: storedProject(3) }))
    renderAt(`/admin/projects/${projectID}/edit`)
    await waitFor(() => expect((screen.getByLabelText('Project title') as HTMLInputElement).value).toBe('Stored Form Project'))

    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    await waitFor(() => expect(apiMocks.replaceProject).toHaveBeenLastCalledWith(expect.objectContaining({ body: expect.objectContaining({ expected_revision: 2 }) })))
    await screen.findByText('The record was saved.')

    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    await waitFor(() => expect(apiMocks.replaceProject).toHaveBeenLastCalledWith(expect.objectContaining({ body: expect.objectContaining({ expected_revision: 3 }) })))
  })

  it('offers explicit reload for a lifecycle revision conflict without discarding form input', async () => {
    const user = userEvent.setup()
    apiMocks.deleteProject.mockRejectedValueOnce({ code: 'revision_conflict', status: 409, title: 'Revision conflict', type: 'about:blank', request_id: 'test', current_revision: 3 })
    renderAt(`/admin/projects/${projectID}/edit`)
    const titleField = await screen.findByLabelText('Project title')
    await waitFor(() => expect((titleField as HTMLInputElement).value).toBe('Stored Form Project'))
    await user.clear(titleField)
    await user.type(titleField, 'Unsaved Local Edit')
    expect((screen.getByLabelText('Project title') as HTMLInputElement).value).toBe('Unsaved Local Edit')

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    await user.type(screen.getByLabelText('Confirmation value'), 'REF-FORM-1')
    await user.click(screen.getByRole('dialog').querySelectorAll('button')[0])

    expect(await screen.findByText('This record changed elsewhere. Unsaved changes remain in this form.')).toBeTruthy()
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.activeElement === screen.getByRole('button', { name: 'Delete' })).toBe(true)
    expect((screen.getByLabelText('Project title') as HTMLInputElement).value).toBe('Unsaved Local Edit')

    apiMocks.getAdminProject.mockResolvedValue({ data: storedProject(3) })
    await user.click(screen.getAllByRole('button', { name: 'Reload current record' })[0])
    await waitFor(() => expect((screen.getByLabelText('Project title') as HTMLInputElement).value).toBe('Stored Form Project'))
  })

  it('keeps the confirmation dialog open while deletion is pending', async () => {
    const user = userEvent.setup()
    apiMocks.deleteProject.mockReturnValue(new Promise(() => {}))
    renderAt(`/admin/projects/${projectID}/edit`)
    await screen.findByLabelText('Project title')

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    const dialog = screen.getByRole('dialog')
    await user.type(screen.getByLabelText('Confirmation value'), 'REF-FORM-1')
    await user.click(dialog.querySelectorAll('button')[0])
    await screen.findByText('Working…')

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.getByRole('dialog')).toBeTruthy()
    fireEvent.pointerDown(document.querySelector('[class*="dialogBackdrop"]') as HTMLElement, {})
    expect(screen.getByRole('dialog')).toBeTruthy()
  })
})
