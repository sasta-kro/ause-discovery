// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it } from 'vitest'
import i18n from '../../app/i18n'
import { ArtifactManagement } from '../../app-shell'
import type { Artifact } from '../../api/generated/types.gen'

const activeArtifact: Artifact = {
  id: 'artifact-active', project_id: 'project-1', artifact_type: 'report', display_name: 'Final report', original_filename: 'report.pdf', mime_type: 'application/pdf', byte_count: 2048, status: 'active', revision: 2, created_at: '', updated_at: '', view_url: '/view', download_url: '/download',
}

const deletedArtifact: Artifact = {
  ...activeArtifact, id: 'artifact-deleted', display_name: 'Old report', status: 'deleted', revision: 3, view_url: null, download_url: null,
}

describe('Artifact management', () => {
  it('renders upload, metadata, replacement, delete, and restore controls', () => {
    const queryClient = new QueryClient()
    render(<QueryClientProvider client={queryClient}><I18nextProvider i18n={i18n}><ArtifactManagement projectId="project-1" projectRevision={4} artifacts={[activeArtifact, deletedArtifact]} csrfToken="csrf" /></I18nextProvider></QueryClientProvider>)

    expect(screen.getByRole('button', { name: 'Upload project file' })).toBeTruthy()
    const activePanel = screen.getByRole('group', { name: 'Final report' })
    expect(within(activePanel).getByRole('button', { name: 'Update metadata' })).toBeTruthy()
    expect(within(activePanel).getByRole('button', { name: 'Replace file' })).toBeTruthy()
    expect(within(activePanel).getByRole('button', { name: 'Delete' })).toBeTruthy()
    const deletedPanel = screen.getByRole('group', { name: 'Old report' })
    expect(within(deletedPanel).getByRole('button', { name: 'Restore' })).toBeTruthy()
  })
})
