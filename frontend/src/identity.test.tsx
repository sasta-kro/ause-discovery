// @vitest-environment jsdom
import { cleanup, fireEvent, render, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProjectDetailIdentity, ProjectIdentity } from './app-shell'
import { useDecodedLogo } from './decoded-logo'

afterEach(cleanup)

type DecodeController = { resolve: () => void; reject: () => void }

// mockDecode replaces HTMLImageElement.decode with controllable promises so
// the reveal boundary stays deterministic without timers.
function mockDecode(): DecodeController[] {
  const controllers: DecodeController[] = []
  Object.defineProperty(HTMLImageElement.prototype, 'decode', {
    configurable: true,
    value: () =>
      new Promise<void>((resolve, reject) => {
        controllers.push({ resolve, reject })
      }),
  })
  return controllers
}

function restoreDecode() {
  delete (HTMLImageElement.prototype as { decode?: unknown }).decode
}

function concealed(image: HTMLImageElement) {
  return image.className.includes('logoConcealed')
}

function revealed(image: HTMLImageElement) {
  return image.className.includes('logoRevealed')
}

describe('Project identity decoded Logo loading', () => {
  it('renders initials without an image when no Logo exists', () => {
    const { container } = render(<ProjectIdentity title="Web Portal" />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.textContent).toBe('WP')
  })

  it('keeps initials visible and the image hidden while the Logo is pending', () => {
    const { container } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
    const image = container.querySelector('img') as HTMLImageElement
    expect(image).not.toBeNull()
    expect(image.getAttribute('alt')).toBe('')
    expect(image.getAttribute('src')).toBe('/p/logo?v=1')
    expect(concealed(image)).toBe(true)
    expect(container.textContent).toContain('WP')
  })

  it('reveals the image only after load and a successful decode', async () => {
    const controllers = mockDecode()
    try {
      const { container } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
      const image = container.querySelector('img') as HTMLImageElement
      fireEvent.load(image)
      // The load event alone must not reveal the Logo.
      expect(concealed(image)).toBe(true)
      controllers[0].resolve()
      await waitFor(() => expect(revealed(image)).toBe(true))
      // Initials remain in the tree beneath the revealed image.
      expect(container.textContent).toContain('WP')
    } finally {
      restoreDecode()
    }
  })

  it('treats a decode rejection like a failed request', async () => {
    const controllers = mockDecode()
    try {
      const { container } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
      fireEvent.load(container.querySelector('img') as HTMLImageElement)
      controllers[0].reject()
      await waitFor(() => expect(container.querySelector('img')).toBeNull())
      expect(container.textContent).toBe('WP')
    } finally {
      restoreDecode()
    }
  })

  it('keeps initials without a broken-image icon when the request fails', () => {
    const { container } = render(<ProjectIdentity logoUrl="/broken?v=1" title="Chatbot System" />)
    fireEvent.error(container.querySelector('img') as HTMLImageElement)
    expect(container.querySelector('img')).toBeNull()
    expect(container.textContent).toBe('CS')
  })

  it('returns to pending when the versioned Logo URL changes', () => {
    const { container, rerender } = render(<ProjectDetailIdentity logoUrl="/logo?v=1" title="Web Portal" />)
    fireEvent.error(container.querySelector('img') as HTMLImageElement)
    expect(container.querySelector('img')).toBeNull()
    rerender(<ProjectDetailIdentity logoUrl="/logo?v=2" title="Web Portal" />)
    const image = container.querySelector('img') as HTMLImageElement
    expect(image.getAttribute('src')).toBe('/logo?v=2')
    expect(concealed(image)).toBe(true)
  })

  it('mounts a fresh image element when the versioned URL changes', () => {
    const { container, rerender } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
    const first = container.querySelector('img') as HTMLImageElement
    rerender(<ProjectIdentity logoUrl="/p/logo?v=2" title="Web Portal" />)
    const second = container.querySelector('img') as HTMLImageElement
    expect(second).not.toBe(first)
    expect(second.getAttribute('src')).toBe('/p/logo?v=2')
    expect(concealed(second)).toBe(true)
  })

  it('ignores a stale request settlement arriving for a replacement URL', async () => {
    const controllers = mockDecode()
    try {
      // Hook level: the first render's handlers are captured, the URL then
      // changes, and the stale handlers are invoked exactly as a late
      // browser event from the previous request would invoke them.
      const initial = renderHook(({ url }) => useDecodedLogo(url), { initialProps: { url: '/p/logo?v=1' } })
      const stale = initial.result.current.imageProps
      // Stand in for the mounted img element the ref would normally hold.
      stale.ref(document.createElement('img'))
      initial.rerender({ url: '/p/logo?v=2' })
      expect(initial.result.current.status).toBe('pending')

      // A late load, its decode success, and a late error from the previous
      // request can none of them settle or fail the replacement URL.
      stale.onLoad()
      expect(controllers.length).toBeGreaterThanOrEqual(1)
      controllers[0].resolve()
      await waitFor(() => expect(initial.result.current.status).toBe('pending'))
      stale.onError()
      await Promise.resolve()
      expect(initial.result.current.status).toBe('pending')

      // The replacement settles only through its own handlers.
      initial.result.current.imageProps.onLoad()
      controllers[controllers.length - 1].resolve()
      await waitFor(() => expect(initial.result.current.status).toBe('ready'))
    } finally {
      restoreDecode()
    }
  })

  it('ignores a late decode rejection from a previous URL', async () => {
    const controllers = mockDecode()
    try {
      const initial = renderHook(({ url }) => useDecodedLogo(url), { initialProps: { url: '/p/logo?v=1' } })
      initial.result.current.imageProps.ref(document.createElement('img'))
      initial.result.current.imageProps.onLoad()
      initial.rerender({ url: '/p/logo?v=2' })
      controllers[0].reject()
      await Promise.resolve()
      expect(initial.result.current.status).toBe('pending')
      initial.result.current.imageProps.ref(document.createElement('img'))
      initial.result.current.imageProps.onLoad()
      controllers[1].resolve()
      await waitFor(() => expect(initial.result.current.status).toBe('ready'))
    } finally {
      restoreDecode()
    }
  })

  it('routes an already-complete cached image through the decode path', async () => {
    const controllers = mockDecode()
    const described = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'complete')
    const widthDescribed = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'naturalWidth')
    Object.defineProperty(HTMLImageElement.prototype, 'complete', { configurable: true, get: () => true })
    Object.defineProperty(HTMLImageElement.prototype, 'naturalWidth', { configurable: true, get: () => 64 })
    try {
      // The mounted image reports already complete, as an image restored
      // from the immutable cache does before the load listener attaches.
      const { container } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
      expect(controllers.length).toBe(1)
      controllers[0].resolve()
      await waitFor(() => expect(revealed(container.querySelector('img') as HTMLImageElement)).toBe(true))
    } finally {
      restoreDecode()
      if (described) Object.defineProperty(HTMLImageElement.prototype, 'complete', described)
      if (widthDescribed) Object.defineProperty(HTMLImageElement.prototype, 'naturalWidth', widthDescribed)
    }
  })

  it('applies search priority hints by cohort', () => {
    const { container: eagerContainer } = render(<ProjectIdentity eager logoUrl="/p/logo?v=1" title="First" />)
    const eagerImage = eagerContainer.querySelector('img') as HTMLImageElement
    expect(eagerImage.getAttribute('loading')).toBe('eager')
    expect(eagerImage.getAttribute('fetchpriority')).toBeNull()

    const { container: lazyContainer } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Later" />)
    const lazyImage = lazyContainer.querySelector('img') as HTMLImageElement
    expect(lazyImage.getAttribute('loading')).toBe('lazy')
    expect(lazyImage.getAttribute('decoding')).toBe('async')
  })

  it('gives the detail header eager loading with high fetch priority', () => {
    const { container } = render(<ProjectDetailIdentity logoUrl="/logo?v=1" title="Web Portal" />)
    const image = container.querySelector('img') as HTMLImageElement
    expect(image.getAttribute('loading')).toBe('eager')
    expect(image.getAttribute('decoding')).toBe('async')
    expect(image.getAttribute('fetchpriority')).toBe('high')
  })

  it('keeps the detail identity decorative with a colored initials fallback', () => {
    const { container } = render(<ProjectDetailIdentity title="Greenhouse Monitor" />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.firstElementChild?.className).toContain('pageHeaderIdentityPurple')
    expect(container.textContent).toBe('GM')
  })

  it('honors the red identity background variant on the detail identity', () => {
    const { container } = render(<ProjectDetailIdentity title="Web Portal" variantClass="pageHeaderIdentityRed" />)
    expect(container.firstElementChild?.className).toContain('pageHeaderIdentityRed')
  })

  it('does not update state when a decode settles after unmount', async () => {
    const controllers = mockDecode()
    try {
      const { container, unmount } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
      fireEvent.load(container.querySelector('img') as HTMLImageElement)
      unmount()
      controllers[0].resolve()
      await Promise.resolve()
    } finally {
      restoreDecode()
    }
  })
})
