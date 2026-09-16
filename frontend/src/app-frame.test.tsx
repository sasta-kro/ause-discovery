// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { I18nextProvider } from 'react-i18next'
import { MemoryRouter, useLocation, useNavigate } from 'react-router'
import { StrictMode, type ReactNode } from 'react'
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
    // jsdom has no scroll implementation; the route policy calls scrollTo on
    // pathname navigation, so it is stubbed for the whole shell file.
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  })
  afterEach(() => {
    vi.restoreAllMocks()
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

  describe('route transition scroll policy', () => {
    let scrollTo: ReturnType<typeof vi.spyOn>
    let focus: ReturnType<typeof vi.spyOn>

    beforeEach(() => {
      scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
      focus = vi.spyOn(HTMLElement.prototype, 'focus')
    })
    afterEach(() => {
      scrollTo.mockRestore()
      focus.mockRestore()
    })

    it('does not focus or scroll on initial rendering', () => {
      renderAt('/')
      expect(focus).not.toHaveBeenCalledWith({ preventScroll: true })
      expect(scrollTo).not.toHaveBeenCalled()
    })

    it('does not focus or scroll on initial rendering under the real StrictMode replay', () => {
      const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
      render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <MemoryRouter initialEntries={['/']}>
              <StrictMode><SessionProvider><AppRoutes /></SessionProvider></StrictMode>
            </MemoryRouter>
          </I18nextProvider>
        </QueryClientProvider>,
      )
      expect(focus).not.toHaveBeenCalledWith({ preventScroll: true })
      expect(scrollTo).not.toHaveBeenCalled()
    })

    it('focuses main with preventScroll and requests position zero on a different pathname', async () => {
      const user = userEvent.setup()
      renderAt('/')
      await user.click(screen.getByRole('link', { name: 'Browse all projects' }))
      await waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 0))
      expect(focus).toHaveBeenCalledWith({ preventScroll: true })
      expect(document.activeElement?.id).toBe('main-content')
    })

    it('returns to the landing page at position zero through the brand link', async () => {
      const user = userEvent.setup()
      renderAt('/search')
      await user.click(screen.getByRole('link', { name: 'AUSE Discovery' }))
      await waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 0))
      expect(focus).toHaveBeenCalledWith({ preventScroll: true })
      expect(document.activeElement?.id).toBe('main-content')
    })

    it('returns the landing page to position zero when the brand activates on Home', async () => {
      const user = userEvent.setup()
      renderAt('/')
      scrollTo.mockClear()
      await user.click(screen.getByRole('link', { name: 'AUSE Discovery' }))
      expect(scrollTo).toHaveBeenCalledWith(0, 0)
      expect(focus).not.toHaveBeenCalledWith({ preventScroll: true })
    })

    it('keeps scroll position on a same-path Search query transition', async () => {
      const user = userEvent.setup()
      let location = ''
      renderAt('/search?q=vision', (value) => { location = value })
      await screen.findByRole('heading', { name: 'Search projects' })
      scrollTo.mockClear()
      focus.mockClear()
      const input = screen.getByLabelText('Search terms') as HTMLInputElement
      await user.clear(input)
      await user.type(input, 'neural')
      await user.click(screen.getByRole('button', { name: 'Search projects' }))
      await waitFor(() => expect(location).toBe('/search?q=neural'))
      expect(scrollTo).not.toHaveBeenCalled()
      expect(focus).not.toHaveBeenCalledWith({ preventScroll: true })
    })

    it('keeps the restored scroll position on history POP navigation', async () => {
      const user = userEvent.setup()
      let goBack: ((delta: number) => void) | undefined
      function NavigationProbe() {
        goBack = useNavigate()
        return null
      }
      const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
      render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <MemoryRouter initialEntries={['/search']}>
              <SessionProvider><NavigationProbe /><AppRoutes /></SessionProvider>
            </MemoryRouter>
          </I18nextProvider>
        </QueryClientProvider>,
      )
      await screen.findByRole('heading', { name: 'Search projects' })
      await user.click(screen.getByRole('link', { name: 'AUSE Discovery' }))
      await waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 0))
      scrollTo.mockClear()
      focus.mockClear()
      // Browser Back is a POP navigation with a pathname change.
      goBack?.(-1)
      await waitFor(() => expect(focus).toHaveBeenCalledWith({ preventScroll: true }))
      expect(scrollTo).not.toHaveBeenCalled()
    })

    it('leaves the skip link as a native hash anchor', () => {
      renderAt('/')
      const skipLink = screen.getByRole('link', { name: 'Skip to main content' })
      expect(skipLink.getAttribute('href')).toBe('#main-content')
      expect(skipLink.tagName).toBe('A')
    })
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
