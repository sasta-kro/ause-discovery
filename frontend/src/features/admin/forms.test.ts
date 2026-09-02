import { describe, expect, it } from 'vitest'
import type { RevisionConflictProblem } from '../../api/generated/types.gen'
import { preserveDraftOnConflict, projectDeleteConfirmation, projectDraftSchema, toProjectDraft, toProjectFormValues } from './forms'

describe('project draft handling', () => {
  const values = { referenceCode: '', title: '', abstract: '', academicYear: '', semester: '', programVersionId: '', majorVersionId: '', courseVersionId: '', studentPersonIds: [], advisorPersonIds: [], coAdvisorPersonIds: [], committeePersonIds: [], categoryTaxonomyIds: [], platformTaxonomyIds: [], domainTaxonomyIds: [], topicTaxonomyIds: [], technologyTaxonomyIds: [] } as const

  it('permits an incomplete draft while validating invalid year input', () => {
    expect(projectDraftSchema.safeParse(values).success).toBe(true)
    expect(projectDraftSchema.safeParse({ ...values, academicYear: 'twenty' }).success).toBe(false)
    expect(toProjectDraft(values).title).toBeNull()
  })

  it('preserves unsaved field values with the server conflict detail', () => {
    const conflict = { code: 'revision_conflict', current_revision: 3, type: '', title: '', status: 409, request_id: '' } as RevisionConflictProblem
    expect(preserveDraftOnConflict({ ...values, title: 'Unsaved title' }, conflict)).toMatchObject({ values: { title: 'Unsaved title' }, conflict: { current_revision: 3 } })
  })

  it('uses Project-specific identity for deletion confirmation', () => {
    expect(projectDeleteConfirmation({ reference_code: 'REF-001', title: 'Project title' })).toBe('REF-001')
    expect(projectDeleteConfirmation({ reference_code: null, title: 'Project title' })).toBe('Project title')
  })

  it('maps role and taxonomy selections into ordered aggregate collections', () => {
    const draft = toProjectDraft({
      ...values,
      studentPersonIds: ['00000000-0000-0000-0000-000000000001'],
      advisorPersonIds: ['00000000-0000-0000-0000-000000000002'],
      coAdvisorPersonIds: [],
      committeePersonIds: ['00000000-0000-0000-0000-000000000003'],
      categoryTaxonomyIds: ['00000000-0000-0000-0000-000000000101'],
      platformTaxonomyIds: ['00000000-0000-0000-0000-000000000111'],
      domainTaxonomyIds: [],
      topicTaxonomyIds: [],
      technologyTaxonomyIds: ['00000000-0000-0000-0000-000000000151'],
    })

    expect(draft.participations).toEqual([
      { person_id: '00000000-0000-0000-0000-000000000001', role: 'student', sort_order: 0 },
      { person_id: '00000000-0000-0000-0000-000000000002', role: 'advisor', sort_order: 0 },
      { person_id: '00000000-0000-0000-0000-000000000003', role: 'committee_member', sort_order: 0 },
    ])
    expect(draft.taxonomy_values).toEqual([
      { taxonomy_value_id: '00000000-0000-0000-0000-000000000101', sort_order: 0 },
      { taxonomy_value_id: '00000000-0000-0000-0000-000000000111', sort_order: 0 },
      { taxonomy_value_id: '00000000-0000-0000-0000-000000000151', sort_order: 0 },
    ])
  })

  it('hydrates role and taxonomy selections from an administrator Project', () => {
    const hydrated = toProjectFormValues({
      reference_code: null,
      title: 'Project title',
      abstract: null,
      academic_year: null,
      semester: null,
      program: null,
      major: null,
      course: null,
      participations: [
        { person: { id: '00000000-0000-0000-0000-000000000001', display_name: 'Student' }, role: 'student', sort_order: 0 },
        { person: { id: '00000000-0000-0000-0000-000000000002', display_name: 'Advisor' }, role: 'advisor', sort_order: 0 },
      ],
      taxonomy: [
        { id: '00000000-0000-0000-0000-000000000101', dimension: 'category', key: 'software_application', labels: { en: 'Software / Application' }, sort_order: 0 },
        { id: '00000000-0000-0000-0000-000000000111', dimension: 'platform', key: 'web', labels: { en: 'Web' }, sort_order: 0 },
      ],
    })

    expect(hydrated.studentPersonIds).toEqual(['00000000-0000-0000-0000-000000000001'])
    expect(hydrated.advisorPersonIds).toEqual(['00000000-0000-0000-0000-000000000002'])
    expect(hydrated.categoryTaxonomyIds).toEqual(['00000000-0000-0000-0000-000000000101'])
    expect(hydrated.platformTaxonomyIds).toEqual(['00000000-0000-0000-0000-000000000111'])
  })
})
