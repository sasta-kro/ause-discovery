// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { AdminPeopleList } from './people-list'

const apiMocks = vi.hoisted(() => ({
  listAdminPeople: vi.fn(),
  createPerson: vi.fn(),
}))

vi.mock('../../api/generated/sdk.gen', () => apiMocks)

const basePerson = {
  id: '018f0000-0000-7000-8000-0000000000b1',
  display_name: 'Ada Advisor',
  student_id: '7770001',
  staff_id: null,
  revision: 1,
  created_at: '2026-09-04T00:00:00Z',
  updated_at: '2026-09-04T00:00:00Z',
}

function renderList() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter><AdminPeopleList csrfToken="csrf-token" /></MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  )
}

describe('administrator people list', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.listAdminPeople.mockImplementation(async ({ query }: { query?: { cursor?: string; q?: string } }) => {
      if (query?.cursor === 'people-cursor-2') return { data: { items: [{ ...basePerson, id: '018f0000-0000-7000-8000-0000000000b2', display_name: 'Bo Committee', student_id: null }], page: { limit: 20 } } }
      return { data: { items: [basePerson], page: { limit: 20, next_cursor: 'people-cursor-2' } } }
    })
  })

  it('validates new People before submission and reports server failures accessibly', async () => {
    const user = userEvent.setup()
    apiMocks.createPerson.mockRejectedValue({ code: 'internal_error', status: 500, title: 'Error', type: 'about:blank', request_id: 'test' })
    renderList()
    expect(await screen.findByText('Ada Advisor')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Create' }))
    expect(await screen.findByText('This field is required.')).toBeTruthy()
    expect(apiMocks.createPerson).not.toHaveBeenCalled()

    await user.type(screen.getByLabelText('Display name'), 'Invalid Person')
    await user.type(screen.getByLabelText('Student ID'), '12345')
    await user.click(screen.getByRole('button', { name: 'Create' }))
    expect(await screen.findByText('Student ID must contain exactly seven digits.')).toBeTruthy()
    expect(apiMocks.createPerson).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText('Student ID'))
    await user.type(screen.getByLabelText('Student ID'), '7770009')
    await user.click(screen.getByRole('button', { name: 'Create' }))
    const failure = await screen.findByRole('alert')
    expect(failure.textContent).toContain('The Person could not be saved.')
    expect(apiMocks.createPerson).toHaveBeenCalledWith(expect.objectContaining({ body: { display_name: 'Invalid Person', student_id: '7770009', staff_id: null }, headers: { 'X-CSRF-Token': 'csrf-token' } }))
  })

  it('filters and pages People with local cursor history', async () => {
    const user = userEvent.setup()
    renderList()
    expect(await screen.findByText('Ada Advisor')).toBeTruthy()

    await user.type(screen.getByLabelText('Filter by name or identifier'), 'Ada')
    await user.click(screen.getByRole('button', { name: 'Apply filters' }))
    await waitFor(() => expect(apiMocks.listAdminPeople).toHaveBeenCalledWith(expect.objectContaining({ query: { q: 'Ada', cursor: undefined, limit: 20 } })))

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(apiMocks.listAdminPeople).toHaveBeenCalledWith(expect.objectContaining({ query: { q: 'Ada', cursor: 'people-cursor-2', limit: 20 } })))
    expect(await screen.findByText('Bo Committee')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Previous page' }))
    expect(await screen.findByText('Ada Advisor')).toBeTruthy()
  })
})
