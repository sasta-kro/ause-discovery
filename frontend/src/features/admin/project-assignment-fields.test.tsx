// @vitest-environment jsdom
import { render, screen, within } from '@testing-library/react'
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it } from 'vitest'
import i18n from '../../app/i18n'
import { ProjectAssignmentFields } from '../../app-shell'
import { toProjectFormValues } from './forms'

function AssignmentFixture() {
  const form = useForm({ defaultValues: toProjectFormValues() })
  return <I18nextProvider i18n={i18n}><ProjectAssignmentFields form={form} people={[{ id: 'person-1', display_name: 'Student Example', student_id: '0123456', staff_id: null, revision: 1, created_at: '', updated_at: '' }]} taxonomy={[{ id: 'category-1', dimension: 'category', key: 'software_application', labels: { en: 'Software / Application' }, sort_order: 1 }, { id: 'topic-1', dimension: 'topic', key: 'computer_vision', labels: { en: 'Computer Vision' }, sort_order: 1 }]} /></I18nextProvider>
}

describe('Project assignment fields', () => {
  it('renders role and taxonomy selectors from available records', () => {
    render(<AssignmentFixture />)

    const students = screen.getByRole('listbox', { name: 'Students' })
    expect(within(students).getByRole('option', { name: 'Student Example (0123456)' })).toBeTruthy()
    const categories = screen.getByRole('listbox', { name: 'Categories' })
    expect(within(categories).getByRole('option', { name: 'Software / Application' })).toBeTruthy()
    expect(screen.queryByRole('listbox', { name: 'Topics' })).toBeNull()
  })
})
