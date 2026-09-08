// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from './app/i18n'
import { AppRoutes, SessionProvider } from './app-shell'

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn(),
  getPublicProject: vi.fn(),
}))

vi.mock('./api/generated/sdk.gen', () => apiMocks)

const projectID = '018f0000-0000-7000-8000-0000000000p1'
const catalog = (id: string, label: string) => ({ id, key: label.toLowerCase(), label })

const publicProject = {
  id: projectID,
  reference_code: 'REF-2026-001',
  title: 'Long Project Title About Discovery',
  abstract: 'An abstract describing the Project.',
  academic_year: 2026,
  semester: 'first',
  program: catalog('018f0000-0000-7000-8000-0000000000p2', 'Computing'),
  major: null,
  course: catalog('018f0000-0000-7000-8000-0000000000p3', 'Capstone'),
  status: 'published',
  taxonomy: [],
  participations: [
    { person: { id: '018f0000-0000-7000-8000-0000000000s1', display_name: 'Sam Student', student_id: '7770001' }, role: 'student', sort_order: 0 },
    { person: { id: '018f0000-0000-7000-8000-0000000000s2', display_name: 'Alex Advisor', student_id: null }, role: 'advisor', sort_order: 0 },
    { person: { id: '018f0000-0000-7000-8000-0000000000s3', display_name: 'Cara Coadvisor', student_id: null }, role: 'co_advisor', sort_order: 0 },
  ],
  artifacts: [
    { id: '018f0000-0000-7000-8000-0000000000a1', project_id: projectID, artifact_type: 'report', display_name: 'Final report', original_filename: 'report.pdf', mime_type: 'application/pdf', byte_count: 5 * 1024 * 1024, status: 'active', revision: 1, view_url: '/ause-discovery/api/v1/artifacts/018f0000-0000-7000-8000-0000000000a1/view', download_url: '/ause-discovery/api/v1/artifacts/018f0000-0000-7000-8000-0000000000a1/download', created_at: '2026-09-04T00:00:00Z', updated_at: '2026-09-04T00:00:00Z', deleted_at: null },
  ],
  extension_metadata: {},
  created_at: '2026-09-04T00:00:00Z',
  updated_at: '2026-09-04T00:00:00Z',
  published_at: '2026-09-04T00:00:00Z',
}

function renderAt(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
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

describe('public project presentation', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getSession.mockRejectedValue({ code: 'unauthorized', status: 401, title: 'Unauthorized', type: 'about:blank', request_id: 'test' })
    apiMocks.getPublicProject.mockResolvedValue({ data: publicProject })
  })

  it('localizes participation roles and opens PDF View in a new browsing context', async () => {
    renderAt(`/projects/${projectID}`)
    expect(await screen.findByRole('heading', { name: 'Long Project Title About Discovery' })).toBeTruthy()
    expect(screen.queryByText('REF-2026-001')).toBeNull()
    expect(screen.getByText('Student')).toBeTruthy()
    expect(screen.getByText('Advisor')).toBeTruthy()
    expect(screen.getByText('Co-advisor')).toBeTruthy()
    expect(screen.queryByText('co_advisor')).toBeNull()
    expect(screen.queryByText('advisor')).toBeNull()

    const viewLink = screen.getByRole('link', { name: 'View' }) as HTMLAnchorElement
    expect(viewLink.target).toBe('_blank')
    expect(viewLink.rel).toContain('noopener')
    expect(viewLink.rel).toContain('noreferrer')
    const downloadLink = screen.getByRole('link', { name: 'Download' }) as HTMLAnchorElement
    expect(downloadLink.target).toBe('')
  })

  it('distinguishes a missing record from a temporary failure', async () => {
    apiMocks.getPublicProject.mockRejectedValue({ code: 'not_found', status: 404, title: 'Not found', type: 'about:blank', request_id: 'test' })
    renderAt(`/projects/${projectID}`)
    expect(await screen.findByRole('heading', { name: 'The requested page was not found.' })).toBeTruthy()
    cleanup()

    apiMocks.getPublicProject.mockRejectedValue(new Error('network down'))
    renderAt(`/projects/${projectID}`)
    expect(await screen.findByRole('heading', { name: 'Content unavailable' })).toBeTruthy()
  })
})
