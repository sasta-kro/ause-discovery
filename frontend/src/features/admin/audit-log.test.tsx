// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { AdminAuditLog } from './audit-log'

const apiMocks = vi.hoisted(() => ({
  listAuditEvents: vi.fn(),
}))

vi.mock('../../api/generated/sdk.gen', () => apiMocks)

const actorID = '018f0000-0000-7000-8000-0000000000a1'
const projectID = '018f0000-0000-7000-8000-0000000000b1'

const firstPage = {
  items: [
    { id: '018f0000-0000-7000-8000-000000000001', action: 'project.created', actor_id: actorID, resource_type: 'project', resource_id: projectID, metadata: { count: 2, source: 'api' }, created_at: '2026-09-04T10:00:00Z' },
    { id: '018f0000-0000-7000-8000-000000000002', action: 'login.failure', actor_id: null, resource_type: 'application_user', resource_id: null, metadata: { username: 'unknown-admin' }, created_at: '2026-09-04T09:00:00Z' },
  ],
  page: { limit: 20, next_cursor: 'page-two-cursor' },
}
const secondPage = {
  items: [
    { id: '018f0000-0000-7000-8000-000000000003', action: 'person.created', actor_id: actorID, resource_type: 'person', resource_id: '018f0000-0000-7000-8000-0000000000c1', metadata: {}, created_at: '2026-09-04T08:00:00Z' },
  ],
  page: { limit: 20 },
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}><I18nextProvider i18n={i18n}><AdminAuditLog /></I18nextProvider></QueryClientProvider>)
}

describe('Administrator audit log', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.listAuditEvents.mockImplementation(async ({ query }: { query?: { cursor?: string } }) => {
      if (query?.cursor === 'page-two-cursor') return { data: secondPage }
      return { data: firstPage }
    })
  })

  it('renders events with actor, resource, and metadata, then pages with cursors', async () => {
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText('project.created')).toBeTruthy()
    expect(screen.getByText(actorID)).toBeTruthy()
    expect(screen.getByText('project')).toBeTruthy()
    expect(screen.getByText(projectID)).toBeTruthy()
    expect(screen.getByText('count')).toBeTruthy()
    expect(screen.getByText('2')).toBeTruthy()
    expect(screen.getByText('System or unauthenticated')).toBeTruthy()
    expect(screen.getByText('username')).toBeTruthy()
    expect(screen.getByText('unknown-admin')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(apiMocks.listAuditEvents).toHaveBeenCalledWith(expect.objectContaining({ query: expect.objectContaining({ cursor: 'page-two-cursor' }) })))
    expect(await screen.findByText('person.created')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Previous page' }))
    await waitFor(() => expect(apiMocks.listAuditEvents).toHaveBeenCalledWith(expect.objectContaining({ query: expect.objectContaining({ cursor: undefined }) })))
    expect(await screen.findByText('project.created')).toBeTruthy()
  })

  it('applies exact filters, clears them, and reports request failures accessibly', async () => {
    const user = userEvent.setup()
    renderPage()
    expect(await screen.findByText('project.created')).toBeTruthy()

    await user.type(screen.getByLabelText('Action (exact match)'), 'project.created')
    await user.type(screen.getByLabelText('Actor UUID'), actorID)
    await user.click(screen.getByRole('button', { name: 'Apply filters' }))
    await waitFor(() => expect(apiMocks.listAuditEvents).toHaveBeenCalledWith(expect.objectContaining({ query: expect.objectContaining({ action: 'project.created', actor_id: actorID, cursor: undefined }) })))

    await user.clear(screen.getByLabelText('Action (exact match)'))
    await user.type(screen.getByLabelText('Actor UUID'), 'not-a-uuid')
    await user.click(screen.getByRole('button', { name: 'Apply filters' }))
    const invalidAlert = await screen.findByRole('alert')
    expect(invalidAlert.textContent).toContain('Actor must be a valid UUID.')

    await user.clear(screen.getByLabelText('Actor UUID'))
    await user.click(screen.getByRole('button', { name: 'Clear filters' }))
    await waitFor(() => expect(apiMocks.listAuditEvents).toHaveBeenCalledWith(expect.objectContaining({ query: { action: undefined, actor_id: undefined, cursor: undefined, limit: 20 } })))

    apiMocks.listAuditEvents.mockRejectedValue(new Error('unavailable'))
    await user.type(screen.getByLabelText('Action (exact match)'), 'person.created')
    await user.click(screen.getByRole('button', { name: 'Apply filters' }))
    const failureAlert = await screen.findByText('The audit listing failed. Adjust the filters and retry.')
    expect(failureAlert.textContent).toContain('The audit listing failed. Adjust the filters and retry.')
    expect(failureAlert.getAttribute('role')).toBe('alert')
  })
})
