import { describe, expect, it } from 'vitest'
import { protectedRedirect } from './app-shell'

describe('administrator route protection', () => {
  it('retains the intended protected route for a later login redirect', () => {
    expect(protectedRedirect('/admin/projects/record/edit?tab=metadata')).toBe('/admin/login?next=%2Fadmin%2Fprojects%2Frecord%2Fedit%3Ftab%3Dmetadata')
  })
})
