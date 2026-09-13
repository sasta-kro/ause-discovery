// @vitest-environment jsdom
import { cleanup, fireEvent, render, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProjectDetailIdentity, ProjectIdentity } from './app-shell'

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

  it('ignores a late completion from a previous URL', async () => {
    const controllers = mockDecode()
    try {
      const { container, rerender } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
      fireEvent.load(container.querySelector('img') as HTMLImageElement)
      rerender(<ProjectIdentity logoUrl="/p/logo?v=2" title="Web Portal" />)
      const second = container.querySelector('img') as HTMLImageElement
      expect(second.getAttribute('src')).toBe('/p/logo?v=2')
      expect(concealed(second)).toBe(true)
      // The decode promise belonging to ?v=1 settles late.
      controllers[0].resolve()
      await Promise.resolve()
      expect(concealed(second)).toBe(true)
    } finally {
      restoreDecode()
    }
  })

  it('routes an already-complete cached image through the decode path', async () => {
    const controllers = mockDecode()
    try {
      const { container, rerender } = render(<ProjectIdentity logoUrl="/p/logo?v=1" title="Web Portal" />)
      const image = container.querySelector('img') as HTMLImageElement
      Object.defineProperty(image, 'complete', { configurable: true, value: true })
      Object.defineProperty(image, 'naturalWidth', { configurable: true, value: 64 })
      // A URL change re-runs the effect, which now observes the completed
      // image and enters the decode path without a load event.
      rerender(<ProjectIdentity logoUrl="/p/logo?v=2" title="Web Portal" />)
      expect(controllers.length).toBe(1)
      controllers[0].resolve()
      await waitFor(() => expect(revealed(container.querySelector('img') as HTMLImageElement)).toBe(true))
    } finally {
      restoreDecode()
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
