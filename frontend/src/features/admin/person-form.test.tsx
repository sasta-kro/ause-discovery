// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter, Route, Routes } from 'react-router'
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

describe('administrator person editing', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getAdminPerson.mockResolvedValue({ data: storedPerson })
  })

  it('preserves entered values across a revision conflict and offers an explicit reload', async () => {
    const user = userEvent.setup()
    apiMocks.updatePerson.mockRejectedValueOnce({ code: 'revision_conflict', status: 409, title: 'Revision conflict', type: 'about:blank', request_id: 'test', current_revision: 3 })
      .mockResolvedValue({ data: { ...storedPerson, display_name: 'Stored Name', revision: 3 } })
    renderForm()
    const nameField = await screen.findByLabelText('Display name')
    expect((nameField as HTMLInputElement).value).toBe('Stored Name')

    await user.clear(nameField)
    await user.type(nameField, 'Corrected Name')
    await user.click(screen.getByRole('button', { name: 'Save draft' }))

    const conflict = await screen.findByText('This record changed elsewhere. Unsaved changes remain in this form.')
    expect(conflict).toBeTruthy()
    expect((screen.getByLabelText('Display name') as HTMLInputElement).value).toBe('Corrected Name')

    await user.click(screen.getByRole('button', { name: 'Reload current record' }))
    await waitFor(() => expect(apiMocks.getAdminPerson).toHaveBeenCalledTimes(2))
    expect((screen.getByLabelText('Display name') as HTMLInputElement).value).toBe('Corrected Name')

    await user.click(screen.getByRole('button', { name: 'Save draft' }))
    await waitFor(() => expect(apiMocks.updatePerson).toHaveBeenLastCalledWith(expect.objectContaining({ body: expect.objectContaining({ expected_revision: 2, display_name: 'Corrected Name' }) })))
    expect(await screen.findByText('The record was saved.')).toBeTruthy()
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
