// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { AdminPersonForm } from './person-form'

const apiMocks = vi.hoisted(() => ({
  getAdminPerson: vi.fn(),
  updatePerson: vi.fn(),
}))

vi.mock('../../api/generated/sdk.gen', () => apiMocks)

const personID = '018f0000-0000-7000-8000-0000000000b1'
const storedPerson = {
  id: personID,
  display_name: 'Stored Name',
  student_id: '7770001',
  staff_id: null,
  revision: 2,
  created_at: '2026-09-04T00:00:00Z',
  updated_at: '2026-09-04T00:00:00Z',
}

function renderForm() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={[`/admin/people/${personID}`]}>
          <Routes><Route element={<AdminPersonForm csrfToken="csrf-token" />} path="/admin/people/:personId" /></Routes>
        </MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  )
}

const secondPersonID = '018f0000-0000-7000-8000-0000000000b2'

function renderNavigableForm() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  let navigate: ((to: string) => void) | undefined
  function NavigationProbe() {
    navigate = useNavigate()
    return null
  }
  const view = render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={[`/admin/people/${personID}`]}>
          <NavigationProbe />
          <Routes><Route element={<AdminPersonForm csrfToken="csrf-token" />} path="/admin/people/:personId" /></Routes>
        </MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  )
  return { navigate: (to: string) => navigate?.(to), unmount: view.unmount }
}

describe('administrator person editing', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getAdminPerson.mockImplementation(async () => ({ data: storedPerson }))
  })

  it('reloads the actual server response after a conflict and saves with the refreshed record', async () => {
    const user = userEvent.setup()
    const renamedPerson = { ...storedPerson, display_name: 'Server Renamed Person', revision: 3 }
    apiMocks.getAdminPerson.mockImplementationOnce(async () => ({ data: storedPerson }))
    apiMocks.getAdminPerson.mockImplementationOnce(async () => {
      await new Promise((resolve) => setTimeout(resolve, 150))
      return { data: renamedPerson }
    })
    apiMocks.updatePerson.mockRejectedValueOnce({ code: 'revision_conflict', status: 409, title: 'Revision conflict', type: 'about:blank', request_id: 'test', current_revision: 3 })
      .mockResolvedValue({ data: renamedPerson })
    renderForm()
    const nameField = await screen.findByLabelText('Display name')
    await waitFor(() => expect((nameField as HTMLInputElement).value).toBe('Stored Name'))

    await user.clear(nameField)
    await user.type(nameField, 'Local Edit')
    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    expect(await screen.findByText('This record changed elsewhere. Unsaved changes remain in this form.')).toBeTruthy()
    expect((screen.getByLabelText('Display name') as HTMLInputElement).value).toBe('Local Edit')

    await user.click(screen.getByRole('button', { name: 'Reload current record' }))
    expect((screen.getByRole('button', { name: 'Save draft' }) as HTMLButtonElement).disabled).toBe(true)
    await waitFor(() => expect((screen.getByLabelText('Display name') as HTMLInputElement).value).toBe('Server Renamed Person'), { timeout: 2000 })

    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    await waitFor(() => expect(apiMocks.updatePerson).toHaveBeenLastCalledWith(expect.objectContaining({ body: expect.objectContaining({ expected_revision: 3, display_name: 'Server Renamed Person' }) })))
    expect(await screen.findByText('The record was saved.')).toBeTruthy()
  })

  it('preserves entered values when the reload request itself fails', async () => {
    const user = userEvent.setup()
    apiMocks.getAdminPerson.mockImplementationOnce(async () => ({ data: storedPerson }))
    apiMocks.getAdminPerson.mockRejectedValueOnce(new Error('reload failed'))
    apiMocks.updatePerson.mockRejectedValueOnce({ code: 'revision_conflict', status: 409, title: 'Revision conflict', type: 'about:blank', request_id: 'test', current_revision: 3 })
    renderForm()
    const nameField = await screen.findByLabelText('Display name')
    await waitFor(() => expect((nameField as HTMLInputElement).value).toBe('Stored Name'))

    await user.clear(nameField)
    await user.type(nameField, 'Recoverable Edit')
    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    expect(await screen.findByText('This record changed elsewhere. Unsaved changes remain in this form.')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Reload current record' }))
    const failure = await screen.findByText('The current record could not be reloaded. Entered values were preserved; retry the reload.')
    expect(failure).toBeTruthy()
    expect((screen.getByLabelText('Display name') as HTMLInputElement).value).toBe('Recoverable Edit')
    expect((screen.getByRole('button', { name: 'Save draft' }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('loads the correct record, revision, and target when navigating between People', async () => {
    const user = userEvent.setup()
    const secondPerson = { ...storedPerson, id: secondPersonID, display_name: 'Second Person', student_id: '7000002', revision: 7 }
    const firstRenamed = { ...storedPerson, display_name: 'First Person Renamed', revision: 4 }
    let firstRecordLoaded = false
    apiMocks.getAdminPerson.mockImplementation(async ({ path }: { path?: { person_id?: string } }) => {
      if (path?.person_id === secondPersonID) return { data: secondPerson }
      if (!firstRecordLoaded) {
        firstRecordLoaded = true
        return { data: storedPerson }
      }
      await new Promise((resolve) => setTimeout(resolve, 150))
      return { data: firstRenamed }
    })
    apiMocks.updatePerson.mockRejectedValueOnce({ code: 'revision_conflict', status: 409, title: 'Revision conflict', type: 'about:blank', request_id: 'test', current_revision: 4 })
      .mockResolvedValue({ data: secondPerson })
    const { navigate } = renderNavigableForm()
    const nameField = await screen.findByLabelText('Display name')
    await waitFor(() => expect((nameField as HTMLInputElement).value).toBe('Stored Name'))

    await user.clear(nameField)
    await user.type(nameField, 'First Person Local Edit')
    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    expect(await screen.findByText('This record changed elsewhere. Unsaved changes remain in this form.')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Reload current record' }))
    navigate(`/admin/people/${secondPersonID}`)
    await waitFor(() => expect((screen.getByLabelText('Display name') as HTMLInputElement).value).toBe('Second Person'))
    expect((screen.getByLabelText('Student ID') as HTMLInputElement).value).toBe('7000002')
    expect(screen.queryByText('This record changed elsewhere. Unsaved changes remain in this form.')).toBeNull()

    await waitFor(() => expect((screen.getByLabelText('Display name') as HTMLInputElement).value).toBe('Second Person'), { timeout: 400 })

    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    await waitFor(() => expect(apiMocks.updatePerson).toHaveBeenLastCalledWith(expect.objectContaining({ path: { person_id: secondPersonID }, body: expect.objectContaining({ expected_revision: 7, display_name: 'Second Person' }) })))
  })

  it('reports unexpected edit failures and a missing record distinctly', async () => {
    apiMocks.getAdminPerson.mockRejectedValue(new Error('network down'))
    renderForm()
    const failure = await screen.findByRole('alert')
    expect(failure.textContent).toContain('The request failed.')
    cleanup()

    apiMocks.getAdminPerson.mockRejectedValue({ code: 'not_found', status: 404, title: 'Not found', type: 'about:blank', request_id: 'test' })
    renderForm()
    const missing = await screen.findByRole('alert')
    expect(missing.textContent).toContain('The requested record was not found.')
  })
})
