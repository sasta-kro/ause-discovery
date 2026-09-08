// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter, useLocation } from 'react-router'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import i18n from './app/i18n'
import { client } from './api/generated/client.gen'
import { installSessionExpiryNotification } from './app/session-expiry'
import { AppRoutes, SessionProvider } from './app-shell'

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn(),
  getCsrfToken: vi.fn(),
  login: vi.fn(),
}))

// Session and login calls are mocked; remaining generated calls stay real so
// the client error interceptor and the AdminGuard contract are exercised
// together through the actual generated client.
vi.mock('./api/generated/sdk.gen', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/generated/sdk.gen')>()),
  getSession: apiMocks.getSession,
  getCsrfToken: apiMocks.getCsrfToken,
  login: apiMocks.login,
}))

const unauthorized = { code: 'unauthorized', status: 401, title: 'Unauthorized', type: 'about:blank', request_id: 'test' }
const activeSession = { user: { id: '018f0000-0000-7000-8000-000000000001', username: 'admin', permissions: ['audit.read'] }, expires_at: '2026-09-05T00:00:00Z' }

function Probe({ onLocation }: { onLocation: (value: string) => void }) {
  const location = useLocation()
  onLocation(`${location.pathname}${location.search}`)
  return null
}

function renderAt(path: string, onLocation?: (value: string) => void) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  if (onLocation) installSessionExpiryNotification(() => queryClient.setQueryData(['session'], null))
  const children: ReactNode = onLocation
    ? <SessionProvider><Probe onLocation={onLocation} /><AppRoutes /></SessionProvider>
    : <SessionProvider><AppRoutes /></SessionProvider>
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={[path]}>{children}</MemoryRouter>
      </I18nextProvider>
    </QueryClientProvider>,
  )
}

describe('shared application shell', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getSession.mockRejectedValue(unauthorized)
    apiMocks.getCsrfToken.mockResolvedValue({ data: { token: 'csrf-token' } })
    document.title = ''
  })

  it('renders one main landmark, correct skip link, and named navigation on a public route', () => {
    renderAt('/')
    const main = screen.getByRole('main')
    expect(main.id).toBe('main-content')
    expect(document.querySelectorAll('main')).toHaveLength(1)
    const skipLink = screen.getByRole('link', { name: 'Skip to main content' })
    expect(skipLink.getAttribute('href')).toBe('#main-content')
    const brandLogo = document.querySelector<HTMLImageElement>('img[src$="ause-discover-logo-v1.svg"]')
    expect(brandLogo?.alt).toBe('')
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeTruthy()
    expect(screen.getByRole('navigation', { name: 'Footer navigation' })).toBeTruthy()
  })

  it('keeps the shared frame and a route-specific title on the login route', async () => {
    renderAt('/admin/login')
    expect(document.querySelectorAll('main')).toHaveLength(1)
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeTruthy()
    expect(screen.getByRole('navigation', { name: 'Footer navigation' })).toBeTruthy()
    await waitFor(() => expect(document.title).toContain('Administrator sign in'))
  })

  it('sets a public route title', async () => {
    renderAt('/')
    await waitFor(() => expect(document.title).toContain('AUSE Discovery'))
  })

  it('returns an expired administrator session to login with the intended destination', async () => {
    apiMocks.getSession.mockResolvedValue({ data: activeSession })
    const fetchMock = vi.fn(async () => new Response(JSON.stringify(unauthorized), { status: 401, headers: { 'Content-Type': 'application/json' } }))
    client.setConfig({ baseUrl: 'http://app-frame.test', fetch: fetchMock as unknown as typeof fetch, throwOnError: true })
    let location = ''
    renderAt('/admin/audit?limit=5', (value) => { location = value })
    await waitFor(() => expect(location).toBe('/admin/login?next=%2Fadmin%2Faudit%3Flimit%3D5'))
    expect(screen.getByRole('heading', { name: 'Administrator sign in' })).toBeTruthy()
  })

  it('keeps a failed login on the page with the generic credential failure', async () => {
    const user = userEvent.setup()
    apiMocks.login.mockRejectedValue(unauthorized)
    renderAt('/admin/login')
    await user.type(screen.getByLabelText('Username'), 'someone')
    await user.type(screen.getByLabelText('Password'), 'wrong-password')
    await user.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByRole('alert')).toBeTruthy()
    expect(screen.getByRole('alert').textContent).toContain('The credentials could not be verified.')
    expect(screen.getByRole('heading', { name: 'Administrator sign in' })).toBeTruthy()
  })

  it('renders the pending contact route and a recoverable Not Found page', () => {
    renderAt('/contact')
    expect(screen.getByRole('heading', { name: 'Contact and institutional information' })).toBeTruthy()
    expect(screen.getByText('Institutional contact information is pending approval.')).toBeTruthy()
    cleanup()

    renderAt('/missing-route')
    expect(screen.getByRole('heading', { name: 'The requested page was not found.' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Back to home' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Search projects' })).toBeTruthy()
  })
})
