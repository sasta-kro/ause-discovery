import { describe, expect, it } from 'vitest'
import { normalizePublicBasePath } from './base-path-core'

describe('normalizePublicBasePath', () => {
  it.each([
    ['', '/'],
    ['/', '/'],
    ['ause-discovery', '/ause-discovery/'],
    ['/ause-discovery/', '/ause-discovery/'],
    ['//ause-discovery//admin//', '/ause-discovery/admin/'],
  ])('normalizes %s to %s', (input, expected) => {
    expect(normalizePublicBasePath(input)).toBe(expected)
  })

  it.each(['/ause-discovery/?preview=1', '/ause-discovery/#section'])(
    'rejects non-path input %s',
    (input) => {
      expect(() => normalizePublicBasePath(input)).toThrow()
    },
  )
})
