// @vitest-environment jsdom
import { cleanup, fireEvent, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ProjectDetailIdentity, ProjectIdentity } from './app-shell'

afterEach(cleanup)

describe('Project identity rendering', () => {
  it('renders the logo as a decorative image inside the fixed footprint', () => {
    const { container } = render(<ProjectIdentity logoUrl="/ause-discovery/api/v1/projects/p/logo?v=2" title="Web Portal" />)
    const image = container.querySelector('img') as HTMLImageElement
    expect(image).not.toBeNull()
    expect(image.getAttribute('alt')).toBe('')
    expect(image.getAttribute('src')).toBe('/ause-discovery/api/v1/projects/p/logo?v=2')
  })

  it('falls back to title initials when no logo exists', () => {
    const { container } = render(<ProjectIdentity title="Web Portal" />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.textContent).toBe('WP')
  })

  it('restores initials when the image fails to load', () => {
    const { container } = render(<ProjectIdentity logoUrl="/broken?v=1" title="Chatbot System" />)
    const image = container.querySelector('img') as HTMLImageElement
    expect(image).not.toBeNull()
    fireEvent.error(image)
    expect(container.querySelector('img')).toBeNull()
    expect(container.textContent).toBe('CS')
  })

  it('renders the detail identity with the same decorative rule', () => {
    const { container } = render(<ProjectDetailIdentity logoUrl="/logo?v=1" title="Any" />)
    const image = container.querySelector('img') as HTMLImageElement
    expect(image.getAttribute('alt')).toBe('')
    expect(container.textContent).toBe('')
  })

  it('falls back to initials on the detail identity without a logo', () => {
    const { container } = render(<ProjectDetailIdentity title="Greenhouse Monitor" />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.textContent).toBe('GM')
  })
})
