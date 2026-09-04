// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from '../../app/i18n'
import { AdminSearchMaintenance } from './search-maintenance'

const apiMocks = vi.hoisted(() => ({
  getSearchStatus: vi.fn(),
  getSearchRebuild: vi.fn(),
  reindexProject: vi.fn(),
  createSearchRebuild: vi.fn(),
}))

vi.mock('../../api/generated/sdk.gen', () => apiMocks)

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}><I18nextProvider i18n={i18n}><AdminSearchMaintenance csrfToken="csrf-token" /></I18nextProvider></QueryClientProvider>)
}

describe('Search maintenance', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getSearchStatus.mockResolvedValue({ data: { available: true, pending_count: 2, failed_count: 1, active_rebuild: null } })
    apiMocks.reindexProject.mockResolvedValue({ data: { project_id: '018f0000-0000-7000-8000-000000000001', desired_revision: 5, state: 'pending' } })
    apiMocks.createSearchRebuild.mockResolvedValue({ data: { id: '018f0000-0000-7000-8000-000000000002', state: 'pending', requested_at: '2026-09-02T00:00:00Z', requested_by: '018f0000-0000-7000-8000-000000000003', total_projects: 0, processed_projects: 0 } })
    apiMocks.getSearchRebuild.mockResolvedValue({ data: { id: '018f0000-0000-7000-8000-000000000002', state: 'processing', requested_at: '2026-09-02T00:00:00Z', requested_by: '018f0000-0000-7000-8000-000000000003', total_projects: 10, processed_projects: 4 } })
  })

  it('shows status and invokes project reindex and full rebuild operations', async () => {
    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText('Search service available')).toBeTruthy()
    expect(screen.getByText('2 pending')).toBeTruthy()
    expect(screen.getByText('1 failed')).toBeTruthy()

    await user.type(screen.getByLabelText('Project ID'), '018f0000-0000-7000-8000-000000000001')
    await user.click(screen.getByRole('button', { name: 'Reindex project' }))
    await waitFor(() => expect(apiMocks.reindexProject).toHaveBeenCalledWith(expect.objectContaining({ path: { project_id: '018f0000-0000-7000-8000-000000000001' }, headers: { 'X-CSRF-Token': 'csrf-token' } })))

    await user.click(screen.getByRole('button', { name: 'Rebuild search index' }))
    await waitFor(() => expect(apiMocks.createSearchRebuild).toHaveBeenCalledWith(expect.objectContaining({ headers: { 'X-CSRF-Token': 'csrf-token' } })))
    expect(await screen.findByText('4 of 10 projects processed')).toBeTruthy()
  })
})
