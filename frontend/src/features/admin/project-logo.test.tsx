// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'

afterEach(cleanup)
import i18n from '../../app/i18n'
import { ProjectLogoManagement } from './project-logo'
import type { AdminProject } from '../../api/generated/types.gen'

const baseProject = {
  id: 'project-1', title: 'Logo Project', abstract: 'Abstract', academic_year: 2026, semester: 'first' as const,
  program: { id: 'p', key: 'p', label: 'Program' }, course: { id: 'c', key: 'c', label: 'Course' },
  status: 'published' as const, taxonomy: [], participations: [], artifacts: [], extension_metadata: {},
  revision: 4, created_at: '', updated_at: '', deleted_at: null, published_at: '', logo_url: null,
}

describe('Project logo management', () => {
  it('renders initials fallback and upload control without a logo', () => {
    const queryClient = new QueryClient()
    render(<QueryClientProvider client={queryClient}><I18nextProvider i18n={i18n}><ProjectLogoManagement csrfToken="csrf" project={baseProject as AdminProject} /></I18nextProvider></QueryClientProvider>)
    expect(screen.getByRole('button', { name: 'Upload Project Logo' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Remove Project Logo' })).toBeNull()
    expect(screen.getByText('LP')).toBeTruthy()
  })

  it('renders preview, replace, and remove actions when a logo exists', () => {
    const queryClient = new QueryClient()
    const project = { ...baseProject, logo_url: '/ause-discovery/api/v1/projects/project-1/logo?v=1' } as AdminProject
    render(<QueryClientProvider client={queryClient}><I18nextProvider i18n={i18n}><ProjectLogoManagement csrfToken="csrf" project={project} /></I18nextProvider></QueryClientProvider>)
    expect(screen.getByRole('button', { name: 'Replace Project Logo' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Remove Project Logo' })).toBeTruthy()
    const image = document.querySelector('img') as HTMLImageElement
    expect(image.getAttribute('alt')).toBe('')
    expect(image.getAttribute('src')).toBe('/ause-discovery/api/v1/projects/project-1/logo?v=1')
  })

  it('disables mutations on deleted Projects and hides actions', () => {
    const queryClient = new QueryClient()
    const project = { ...baseProject, status: 'deleted' as const, logo_url: '/logo?v=1' } as AdminProject
    render(<QueryClientProvider client={queryClient}><I18nextProvider i18n={i18n}><ProjectLogoManagement csrfToken="csrf" disabled project={project} /></I18nextProvider></QueryClientProvider>)
    expect(screen.queryByRole('button', { name: 'Replace Project Logo' })).toBeNull()
    expect(screen.getByText('Restore the Project before changing its logo.')).toBeTruthy()
  })
})
